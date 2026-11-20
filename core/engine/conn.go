package engine

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// obfsPacketConn — прослойка между UDP-сокетом и DTLS.
//
// Порядок слоёв на прямом пути (порт 443):
//
//	UDP  →  obfs-RTP  →  DTLS  →  сырой IP-пакет
//
// То есть obfs стоит НИЖЕ DTLS: pion получает на вход уже освобождённые
// от маскировки байты. Именно поэтому шлюз до первого пакета не может
// опознать пароль иначе как перебором ключей — DTLS-сессии ещё нет.
type obfsPacketConn struct {
	inner net.PacketConn

	// dropped — пакеты, не прошедшие проверку целостности.
	//
	// Единственное наше слепое место на приёме: снаружи «пришёл и
	// отброшен» неотличим от «не пришёл». См. ReadFrom.
	dropped atomic.Uint64

	// onRaw — куда отдать пакет, который пришёл НЕ от нашего слоя.
	//
	// Пусто в норме и в релизе. Ставится только под отладочную
	// проверку отображения NAT, см. session.probeStun. Присваивается
	// ДО запуска насосов и больше не меняется — иначе была бы гонка.
	onRaw func([]byte)
	state *obfsState
	rbuf  []byte

	// logf — лог сессии. Ошибка UDP-сокета до нас иначе не доходит:
	// pion, получив её, молча гасит DTLS изнутри, и снаружи это выглядит
	// как EOF при чтении туннеля — без единого намёка на причину.
	logf func(string, ...interface{})

	// wmu делает упаковку и запись в сокет одной неделимой операцией:
	// номер, присвоенный пакету в wrap, и порядок его ухода в сеть
	// совпадают всегда, кто бы ни звал WriteTo — tunToNet, тикер
	// keepalive или внутренние повторы pion.
	//
	// ЧЕСТНАЯ ОГОВОРКА. На сегодняшнем стеке этот замок ничего не
	// чинит: pion зовёт нас через netctx.packetConn, а тот держит
	// собственный writeMu на всё время вызова нижележащего WriteTo
	// (transport/v4@v4.0.2 netctx/packetconn.go:110-112,145), и путь
	// записи там ровно один (dtls/v3@v3.1.5 conn.go:575). То есть наши
	// вызовы уже сериализованы снаружи. Замок оставлен как страховка на
	// случай смены версии pion или второго писателя мимо DTLS: цена —
	// один неоспариваемый мьютекс на пакет, а порядок больше не зависит
	// от чужих внутренностей.
	//
	// Мьютекс СВОЙ, отдельный от obfsState.mu. Порядок взятия всегда
	// один: wmu снаружи, obfsState.mu внутри wrap. Обратного порядка
	// нет нигде — unwrap на приёме obfsState.mu не берёт вовсе, — и
	// поэтому взаимной блокировки быть не может.
	wmu sync.Mutex

	// Ошибка сокета повторяется на КАЖДОМ пакете, поэтому каждое
	// направление докладывает о ней ровно один раз за жизнь соединения.
	// Без этих защёлок получится ровно тот неограниченный поток строк,
	// что на роутере дал 118 МБ за 19 минут.
	readOnce  sync.Once
	writeOnce sync.Once

	// shortOnce — неполная запись в сокет. Для UDP это не должно
	// случаться никогда (датаграмма уходит целиком либо не уходит
	// вовсе), но раньше результат inner.WriteTo просто отбрасывался,
	// и обрезанная запись прошла бы бесследно.
	shortOnce sync.Once

	// sizeOnce — размер ПЕРВОГО упакованного пакета. Ровно та цифра,
	// которой не хватало: сколько байт мы кладём на провод при данном
	// размере полезной нагрузки.
	sizeOnce sync.Once

	// wireMax — самый крупный пакет, ушедший на провод за сессию.
	//
	// Одной строки про ПЕРВУЮ упаковку не хватило: первым уходит AUTH,
	// а это сорок с небольшим байт, и характерного размера мы так и не
	// увидели. Максимум показывает, упёрлись мы в порог пути или нет.
	//
	// atomic: пишется из горутин перекачки, читается из stop().
	wireMax atomic.Uint64

	// retired — слот сняли ПЛАНОВО (автоподбор), и закрытие сокета
	// ожидаемо.
	//
	// Без этого признака плановое снятие писало в лог
	// "UDP-сокет, ошибка чтения: use of closed network connection" —
	// слово "ошибка" о штатном действии. В логе владельца 09.09 таких
	// строк было три подряд, ровно по числу снятых слотов, и выглядели
	// они как поломка. Насос чтения (netToTun) про снятие знал и молчал
	// правильно, а слой под ним — нет.
	retired atomic.Bool

	// pacer выполняет микросекундное сглаживание UDP-всплесков для предотвращения QoS-дропов операторов
	pacer *udpPacer
}

// udpPacer реализует микросекундный шейпинг (token bucket) для устранения пиковых микро-всплесков UDP.
type udpPacer struct {
	enabled         bool
	rateBytesPerSec int64         // 0 = без ограничения полосы (только минимальный интервал)
	burstCapacity   int64         // максимальный размер всплеска в байтах
	tokens          int64         // текущие доступные токены
	minInterval     time.Duration // минимальный интервал между последовательными отправками датаграмм (например, 50 мкс)
	lastSend        time.Time
}

func newUDPPacer(rateBytesPerSec int64, minInterval time.Duration) *udpPacer {
	if minInterval < 0 {
		minInterval = 0
	}
	burst := rateBytesPerSec / 10
	if burst < 65536 {
		burst = 65536
	}
	return &udpPacer{
		enabled:         true,
		rateBytesPerSec: rateBytesPerSec,
		burstCapacity:   burst,
		tokens:          burst,
		minInterval:     minInterval,
		lastSend:        time.Now(),
	}
}

func (p *udpPacer) Pace(packetLen int) {
	if p == nil || !p.enabled {
		return
	}
	now := time.Now()

	// 1. Минимальный микросекундный интервал между последовательными пакетами
	if p.minInterval > 0 && !p.lastSend.IsZero() {
		elapsed := now.Sub(p.lastSend)
		if elapsed < p.minInterval {
			sleepTime := p.minInterval - elapsed
			if sleepTime > 0 && sleepTime < 10*time.Millisecond {
				time.Sleep(sleepTime)
				now = time.Now()
			}
		}
	}

	// 2. Token Bucket для ограничения пиковой полосы при установленном rateBytesPerSec
	if p.rateBytesPerSec > 0 {
		elapsed := now.Sub(p.lastSend)
		if elapsed > 0 {
			refill := int64(elapsed.Seconds() * float64(p.rateBytesPerSec))
			p.tokens += refill
			if p.tokens > p.burstCapacity {
				p.tokens = p.burstCapacity
			}
		}

		needed := int64(packetLen)
		if p.tokens < needed {
			deficit := needed - p.tokens
			sleepSec := float64(deficit) / float64(p.rateBytesPerSec)
			sleepDur := time.Duration(sleepSec * float64(time.Second))
			if sleepDur > 50*time.Millisecond {
				sleepDur = 50 * time.Millisecond
			}
			if sleepDur > 0 {
				time.Sleep(sleepDur)
				now = time.Now()
			}
			p.tokens = 0
		} else {
			p.tokens -= needed
		}
	}

	p.lastSend = now
}

// maxWire — самый крупный пакет после обфускации, без обвязки TURN.
func (c *obfsPacketConn) maxWire() uint64 {
	return c.wireMax.Load()
}

func newObfsPacketConn(inner net.PacketConn, state *obfsState, logf func(string, ...interface{})) *obfsPacketConn {
	return &obfsPacketConn{
		inner: inner,
		state: state,
		rbuf:  make([]byte, 2048),
		logf:  logf,
		pacer: newUDPPacer(0, 50*time.Microsecond),
	}
}

func (c *obfsPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	for {
		n, addr, err := c.inner.ReadFrom(c.rbuf)
		if err != nil {
			c.readOnce.Do(func() {
				if c.logf != nil && !c.retired.Load() {
					c.logf("UDP-сокет, ошибка чтения: %v", err)
				}
			})
			return 0, nil, err
		}
		// ОТВЕТ STUN, ЕСЛИ ЕГО ЖДУТ. Разделяются они однозначно и
		// по построению: у наших пакетов первые два бита 0b10 (версия
		// RTP 2), у STUN — 0b00, плюс волшебное число в байтах 4..8.
		// Ровно так же разделяют STUN и RTP на общем порту в WebRTC.
		//
		// Нужен для одной задачи: увидеть, не переназначил ли
		// операторский NAT наше отображение посреди сессии. Меряться
		// это может ТОЛЬКО с того же сокета — с любого другого
		// отображение своё.
		if c.onRaw != nil && n >= 20 && c.rbuf[0]&0xC0 == 0 &&
			binary.BigEndian.Uint32(c.rbuf[4:8]) == 0x2112A442 {
			raw := make([]byte, n)
			copy(raw, c.rbuf[:n])
			c.onRaw(raw)
			continue
		}

		out, ok := c.state.unwrap(c.rbuf[:n], p)
		if !ok {
			// Мусор или чужой пакет. Молча дальше — но БЕЗ логирования:
			// на открытом порту это давало бы неограниченный поток строк.
			//
			// СЧИТАЕМ, ХОТЬ И МОЛЧИМ. Это наше единственное слепое
			// место: пакет мог прийти и не пройти проверку целостности,
			// и снаружи это неотличимо от «не пришёл вовсе».
			c.dropped.Add(1)
			continue
		}
		if out == 0 {
			// Фиктивный junk-пакет — пропускаем молча и продолжаем приём
			continue
		}
		return out, addr, nil
	}
}

func (c *obfsPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	// Присвоение номера и отправка — под одним замком, см. wmu.
	c.wmu.Lock()
	defer c.wmu.Unlock()

	if c.pacer != nil {
		c.pacer.Pace(len(p))
	}

	wire, err := c.state.wrap(p)
	if err != nil {
		return 0, err
	}

	c.sizeOnce.Do(func() {
		if c.logf != nil {
			// "после обфускации", а не "на проводе": на релейном пути
			// TURN добавит сверху ещё свою обвязку — от 4 байт при
			// канальном связывании до примерно 46 при Send-индикации.
			// Называть это число проводным значило бы врать ровно там,
			// где по нему судят о порогах MTU.
			c.logf("первая упаковка: полезных %d байт, после обфускации %d (+%d)",
				len(p), len(wire), len(wire)-len(p))
		}
	})
	if w := uint64(len(wire)); w > c.wireMax.Load() {
		c.wireMax.Store(w)
	}

	nw, err := c.inner.WriteTo(wire, addr)
	if err != nil {
		c.writeOnce.Do(func() {
			if c.logf != nil {
				c.logf("UDP-сокет, ошибка записи: %v", err)
			}
		})
		return 0, err
	}
	// Сторож обрезки. Молчать тут нельзя: обрезанная датаграмма не
	// пройдёт у шлюза проверку AEAD-тега и будет выброшена, а у нас
	// счётчик отправленных вырастет как ни в чём не бывало.
	//
	// Сравнение именно МЕНЬШЕ, а не "не равно", и вот почему.
	//
	// Под нами не всегда обычный UDP-сокет: на релейном пути это
	// relay-соединение pion/turn, и оно возвращает РАЗНОЕ в зависимости
	// от того, успело ли встать канальное связывание:
	//
	//   internal/client/udp_conn.go:225 — через Send-индикацию:
	//       return c.client.WriteTo(msg.Raw, ...)
	//     то есть длину ВСЕГО STUN-сообщения, вместе с обвязкой TURN;
	//
	//   internal/client/udp_conn.go:234 — через канал:
	//       return len(payload), nil
	//     то есть ровно полезную нагрузку.
	//
	// Первые пакеты идут индикациями, дальше связывание встаёт, и
	// возвращаемое значение меняется на ходу. Отсюда и ложная тревога
	// "записано 132 из 86": 86 наших байт плюс обвязка Send-индикации.
	//
	// Знать про размер обвязки не годится: он зависит от способа
	// отправки, семейства адреса и набивки, и меняется посреди сессии.
	// Выключать сторожа на релее тоже нельзя — там он нужнее всего.
	//
	// А вот "ушло МЕНЬШЕ, чем передали" — это ровно обрезка, и на любом
	// транспорте она означает одно и то же. Больше переданного обрезкой
	// не бывает: обычный sendto никогда не отчитается о лишних байтах,
	// а на релее это просто обвязка.
	if nw < len(wire) {
		c.shortOnce.Do(func() {
			if c.logf != nil {
				c.logf("транспорт: ушло %d из %d байт, пакет обрезан", nw, len(wire))
			}
		})
		return 0, fmt.Errorf("неполная запись: ушло %d из %d байт", nw, len(wire))
	}
	return len(p), nil
}

// InjectJunk отправляет зашифрованный фиктивный RTP-пакет для маскировки перед DPI.
func (c *obfsPacketConn) InjectJunk(addr net.Addr, payloadLen int) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	wire, err := c.state.wrapDummy(payloadLen)
	if err != nil {
		return err
	}
	_, err = c.inner.WriteTo(wire, addr)
	return err
}

// SetPacing настраивает параметры микросекундного шейпинга пакетов.
func (c *obfsPacketConn) SetPacing(rateBytesPerSec int64, minInterval time.Duration) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if rateBytesPerSec == 0 && minInterval <= 0 {
		if c.pacer != nil {
			c.pacer.enabled = false
		}
		return
	}
	c.pacer = newUDPPacer(rateBytesPerSec, minInterval)
}

func (c *obfsPacketConn) Close() error                       { return c.inner.Close() }
func (c *obfsPacketConn) LocalAddr() net.Addr                { return c.inner.LocalAddr() }
func (c *obfsPacketConn) SetDeadline(t time.Time) error      { return c.inner.SetDeadline(t) }
func (c *obfsPacketConn) SetReadDeadline(t time.Time) error  { return c.inner.SetReadDeadline(t) }
func (c *obfsPacketConn) SetWriteDeadline(t time.Time) error { return c.inner.SetWriteDeadline(t) }
