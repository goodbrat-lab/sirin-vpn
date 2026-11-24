package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"sirin/engine/desktop"
)

var Version = "2.2.0"

func main() {
	configPath := flag.String("config", "/etc/sirin/client.toml", "путь к конфигурационному файлу TOML")
	showVersion := flag.Bool("version", false, "показать версию и выйти")

	// Проверяем вызов RPC в формате: sirin-client rpc <method> [input]
	if len(os.Args) > 1 && os.Args[1] == "rpc" {
		method := ""
		if len(os.Args) > 2 {
			method = os.Args[2]
		}
		input := ""
		if len(os.Args) > 3 {
			input = os.Args[3]
		} else {
			input = ReadStdinOrArg("")
		}
		out, code := ExecuteRPC(method, input, *configPath)
		fmt.Println(out)
		os.Exit(code)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("sirin-client v%s (OpenWrt/Linux router client)\n", Version)
		return
	}

	// Проверяем вызов RPC в формате: sirin-client --config /path rpc <method> [input]
	args := flag.Args()
	if len(args) > 0 && args[0] == "rpc" {
		method := ""
		if len(args) > 1 {
			method = args[1]
		}
		input := ""
		if len(args) > 2 {
			input = args[2]
		} else {
			input = ReadStdinOrArg("")
		}
		out, code := ExecuteRPC(method, input, *configPath)
		fmt.Println(out)
		os.Exit(code)
	}

	log.Printf("Запуск Sirin VPN Client v%s...", Version)

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации %s: %v", *configPath, err)
	}

	if cfg.Password == "" {
		log.Fatalf("Ключ доступа (password / UUID) не указан в %s", *configPath)
	}

	// 1. Определение WAN интерфейса и шлюза
	wan := resolveEffectiveWan(cfg)
	if wan.iface == "" {
		log.Printf("[net] ВНИМАНИЕ: WAN-интерфейс не обнаружен автоматически (default route отсутствует)")
	} else {
		log.Printf("[net] Обнаружен WAN: iface=%s, gw=%s", wan.iface, wan.gw)
	}

	// 2. Создание виртуального TUN интерфейса
	tun, err := openTun(cfg.TunName, cfg.TunMTU)
	if err != nil {
		log.Fatalf("Ошибка создания TUN (%s): %v", cfg.TunName, err)
	}
	defer tun.Close()
	log.Printf("[tun] Создан интерфейс %s (MTU=%d)", tun.name, cfg.TunMTU)

	// 3. Сборка лестницы подключений (API или Static Fallback)
	ladder, gateway, serverHosts, err := buildLadder(cfg)
	if err != nil {
		log.Fatalf("Ошибка сборки лестницы подключений: %v", err)
	}
	// На роутере прямой обход DPI уже выполняется через нативный kernel-socket прокси (antidpi.go) и nftables.
	// Отключаем userspace tun_zapret поверх TUN, чтобы не резать пропускную способность (4K YouTube и т.д.).
	ladder.SetLazyZapret(false)
	log.Printf("[ladder] Подключение к шлюзу: %s (хостов для исключения: %d)", gateway, len(serverHosts))

	// Проверка на потенциальный конфликт с sing-box / xray TPROXY
	checkTproxyConflict(serverHosts)

	// 4. Подготовка контекста и сетевого протектора
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	protector := newProtector(wan.iface, wan.gw)

	desktopCfg := desktop.Config{
		Gateway:   gateway,
		Password:  cfg.Password,
		DeviceID:  cfg.DeviceID,
		Ladder:    ladder,
		Protector: protector,
		TunnelDNS: desktop.DefaultTunnelDNS,
		ClampMSS:  true,
		LogFunc: func(line string) {
			log.Printf("[engine] %s", line)
		},
	}

	// 5. Запуск движка через универсальный фасад
	handle, events, err := desktop.Start(ctx, desktopCfg, tun)
	if err != nil {
		log.Fatalf("Ошибка запуска сессии движка: %v", err)
	}
	defer handle.Stop()

	// 6. Перехват системных сигналов для корректного отключения и hotplug-восстановления маршрутов (SIGHUP)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)

	var routeMu sync.Mutex
	var tunnelReady atomic.Bool
	curWan := wan

	runReconcile := func() {
		if !tunnelReady.Load() {
			return
		}
		routeMu.Lock()
		defer routeMu.Unlock()
		if !tunnelReady.Load() {
			return
		}
		nextWan, changed, rerr := reconcileRouting(cfg, tun, handle.Info(), serverHosts, curWan)
		if rerr != nil {
			log.Printf("[route] Предупреждение сверки маршрутов: %v", rerr)
			return
		}
		if nextWan != curWan {
			curWan = nextWan
			protector.UpdateWAN(curWan.iface, curWan.gw)
			log.Printf("[net] Обновлён активный WAN в протекторе: iface=%s, gw=%s", curWan.iface, curWan.gw)
		} else if changed {
			protector.UpdateWAN(curWan.iface, curWan.gw)
		}
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-hupCh:
				log.Printf("[net] Получен сигнал SIGHUP (hotplug/reconcile) — проверка таблиц PBR и WAN...")
				runReconcile()
			case <-ticker.C:
				runReconcile()
			}
		}
	}()

	doneCh := make(chan struct{})
	var antiDPIServer *AntiDPIServer
	var psnProxyServer *PSNProxyServer
	var directWANDNS *DirectWANDNSServer
	var splitDomainDNS *SplitDomainDNSServer
	var forceVPNActive bool
	var dohCanaryActive bool

	stopDnsExtensions := func() {
		changed := false
		if directWANDNS != nil {
			directWANDNS.Stop()
			directWANDNS = nil
		}
		if splitDomainDNS != nil {
			removeDnsmasqDropIn(splitDomainDnsmasqFile)
			splitDomainDNS.Stop()
			splitDomainDNS = nil
			changed = true
		}
		if psnProxyServer != nil {
			removeDnsmasqDropIn(psnProxyDnsmasqFilename)
			psnProxyServer.Stop()
			psnProxyServer = nil
			changed = true
		}
		if antiDPIServer != nil {
			antiDPIServer.Stop()
			removeDnsmasqDropIn(antiDPIDnsmasqFilename)
			antiDPIServer = nil
			changed = true
		}
		if forceVPNActive {
			removeDnsmasqDropIn(forceVPNDnsmasqFilename)
			forceVPNActive = false
			changed = true
		}
		if dohCanaryActive {
			removeDnsmasqDropIn(dohCanaryDnsmasqFilename)
			dohCanaryActive = false
			changed = true
		}
		if cfg.IsDNSFilterEnabled() {
			removeDnsmasqDropIn(dnsFilterDnsmasqFilename)
			changed = true
		}
		if changed {
			restartDnsmasq()
		}
	}

	go func() {
		for ev := range events {
			switch ev.Kind {
			case "ready":
				log.Printf("[engine] Туннель готов: %s", ev.Reason)
				routeMu.Lock()
				curWan = resolveEffectiveWan(cfg)
				protector.UpdateWAN(curWan.iface, curWan.gw)
				if err := setupRouting(cfg, tun, handle.Info(), serverHosts, curWan); err != nil {
					log.Printf("[route] Ошибка настройки маршрутов: %v", err)
				}
				tunnelReady.Store(true)
				routeMu.Unlock()
				needDnsmasqRestart := false
				var warmDomains []string
				// Повторный "ready" (переподключение туннеля): останавливаем прежние серверы
				if psnProxyServer != nil {
					psnProxyServer.Stop()
					psnProxyServer = nil
				}
				if directWANDNS != nil {
					directWANDNS.Stop()
					directWANDNS = nil
				}
				if splitDomainDNS != nil {
					splitDomainDNS.Stop()
					splitDomainDNS = nil
				}
				var psnCfg *PSNProxyConfig
				if sp := cfg.GetServiceProxyConfig(); sp != nil && sp.Enabled != nil && *sp.Enabled {
					psnCfg = sp.ToPSNProxyConfig()
				}
				// Запускаем DNS-форвардер для прямых загрузок игр PS5 (dl.playstation.net -> WAN 0x2),
				// если не выбран режим проксирования загрузок и не отключён RuDirect («Максимальная приватность»).
				wantDirectDL := cfg.SplitTunnel != nil && cfg.SplitTunnel.UseRuDirect != nil && *cfg.SplitTunnel.UseRuDirect
				if psnCfg != nil && psnCfg.IncludeDownloads {
					wantDirectDL = false
				}
				if wantDirectDL {
					directWANDNS = NewDirectWANDNSServer(DefaultDirectWANDNSPort)
					if err := directWANDNS.Start(ctx); err != nil {
						log.Printf("[directwan] Внимание: не удалось запустить DNS-форвардер загрузок: %v", err)
						directWANDNS = nil
					}
				}
				// Убираем «осиротевший» drop-in от прошлого запуска (если процесс был убит)
				removeDnsmasqDropIn(psnProxyDnsmasqFilename)
				removeDnsmasqDropIn(dnsFilterDnsmasqFilename)
				removeDnsmasqDropIn(splitDomainDnsmasqFile)
				removeDnsmasqDropIn(dohCanaryDnsmasqFilename)
				needDnsmasqRestart = true
				if cfg.GetOpSecOptions().BlockDoH {
					_ = writeDnsmasqDropIn(dohCanaryDnsmasqFilename, BuildDnsmasqDoHCanaryContent())
					dohCanaryActive = true
					log.Printf("[opsec] Активировано подавление браузерного DoH (use-application-dns.net -> NXDOMAIN + reject DoH :443)")
				}
				if cfg.GetSplitDomainOptions().Enabled && cfg.SplitTunnel != nil {
					splitDomainDNS = NewSplitDomainDNSServer(DefaultSplitDomainDNSPort, cfg.SplitTunnel.Mode, cfg.SplitTunnel.Domains)
					if err := splitDomainDNS.Start(ctx); err != nil {
						log.Printf("[splitdns] Внимание: не удалось запустить Split DNS: %v", err)
						splitDomainDNS = nil
					} else if content := BuildDnsmasqSplitDomainsContent(cfg.SplitTunnel.Domains, DefaultSplitDomainDNSPort); content != "" {
						_ = writeDnsmasqDropIn(splitDomainDnsmasqFile, content)
						needDnsmasqRestart = true
					}
				}
				if cfg.IsDNSFilterEnabled() {
					_ = writeDnsmasqDropIn(dnsFilterDnsmasqFilename, BuildDnsmasqBlocklistContent(cfg.DNSFilter))
					log.Printf("[dnsfilter] Активирован DNS-фильтр (заблокировано доменов: %d)", len(cfg.DNSFilter.ResolveBlockedDomains()))
				}
				if antiDPIServer != nil {
					antiDPIServer.Stop()
					antiDPIServer = nil
				}
				if psnCfg != nil {
					psnProxyServer = NewPSNProxyServer(*psnCfg, currentDirectMatcher)
					if err := psnProxyServer.Start(ctx); err != nil {
						log.Printf("[psnproxy] Внимание: не удалось запустить PSN-прокси: %v", err)
						psnProxyServer = nil
					} else {
						_ = writeDnsmasqDropIn(psnProxyDnsmasqFilename, BuildDnsmasqPSNProxyContent(psnCfg))
						needDnsmasqRestart = true
						go psnProxyServer.SelfCheck(ctx)
					}
				}
				if cfg.IsAntiDPIEnabled() {
					adCfg := cfg.GetAntiDPIConfig()
					antiDPIServer = NewAntiDPIServer(adCfg)
					if err := antiDPIServer.Start(ctx); err != nil {
						log.Printf("[antidpi] Внимание: не удалось запустить Anti-DPI сервер: %v", err)
					} else {
						adDomains := adCfg.GetTargetDomains()
						_ = WriteDnsmasqAntiDPI(adDomains)
						warmDomains = append(warmDomains, adDomains...)
						needDnsmasqRestart = true
					}
				}
				if cfg.IsForceVPNEnabled() {
					fvOpts := cfg.GetForceVPNOptions()
					fvDomains := cfg.ForceVPN.ResolveTargetDomains()
					if psnProxyServer != nil && psnCfg != nil {
						// Домены PSN обслуживает PSN-прокси — убираем их из ForceVPN
						fvDomains = psnCfg.FilterOutPSNDomains(fvDomains)
					}
					if len(fvDomains) > 0 {
						_ = SetupDnsmasqForceVPN(fvDomains, fvOpts.DNSServer, false)
						warmDomains = append(warmDomains, fvDomains...)
						forceVPNActive = true
						needDnsmasqRestart = true
					}
					log.Printf("[forcevpn] Активирован принудительный VPN (доменов: %d, статических CIDR: %d, DNS: %s)",
						len(fvDomains), len(fvOpts.StaticCIDRs), fvOpts.DNSServer)
				}
				if needDnsmasqRestart {
					restartDnsmasq()
				}
				if len(warmDomains) > 0 {
					go warmForceVPNDomains(ctx, warmDomains)
				}
				if psnProxyServer != nil {
					go psnProxyServer.Warm(ctx)
				}
				if directWANDNS != nil {
					go directWANDNS.Warm(ctx)
				}
				if splitDomainDNS != nil {
					go splitDomainDNS.Warm(ctx)
				}
			case "log":
				// Логи выводятся в реальном времени через desktopCfg.LogFunc
			case "stopping":
				log.Printf("[engine] Остановка: %s", ev.Reason)
			case "stopped":
				tunnelReady.Store(false)
				log.Printf("[engine] Завершено: %s", ev.Reason)
				stopDnsExtensions()
				routeMu.Lock()
				cleanupRouting(cfg, tun, serverHosts, curWan)
				routeMu.Unlock()
				close(doneCh)
				return
			}
		}
	}()

	select {
	case sig := <-sigCh:
		tunnelReady.Store(false)
		log.Printf("Получен сигнал %s, завершение работы...", sig)
		stopDnsExtensions()
		routeMu.Lock()
		cleanupRouting(cfg, tun, serverHosts, curWan)
		routeMu.Unlock()
		handle.Stop()
		cancel()
		tun.Close()
	case <-doneCh:
		log.Println("Сессия туннеля завершена.")
	}
}
