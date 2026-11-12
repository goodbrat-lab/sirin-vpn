package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sirin/engine/desktop"
)

var Version = "1.6.2"

func main() {
	configPath := flag.String("config", "/etc/sirin/client.toml", "путь к конфигурационному файлу TOML")
	showVersion := flag.Bool("version", false, "показать версию и выйти")
	flag.Parse()

	if *showVersion {
		fmt.Printf("sirin-client v%s (OpenWrt/Linux router client)\n", Version)
		return
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
	wan := detectWan()
	if cfg.WanInterface != "" && cfg.WanInterface != "wan" && cfg.WanInterface != "auto" {
		wan.iface = cfg.WanInterface
	} else if wan.iface == "" && cfg.WanInterface != "" {
		wan.iface = cfg.WanInterface
	}
	if cfg.WanGateway != "" {
		wan.gw = cfg.WanGateway
	}
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

	// 6. Перехват системных сигналов для корректного отключения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	doneCh := make(chan struct{})
	var antiDPIServer *AntiDPIServer
	var forceVPNActive bool

	stopDnsExtensions := func() {
		changed := false
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
		if changed {
			restartDnsmasq()
		}
	}

	go func() {
		for ev := range events {
			switch ev.Kind {
			case "ready":
				log.Printf("[engine] Туннель готов: %s", ev.Reason)
				if err := setupRouting(cfg, tun, handle.Info(), serverHosts, wan); err != nil {
					log.Printf("[route] Ошибка настройки маршрутов: %v", err)
				}
				needDnsmasqRestart := false
				if cfg.IsAntiDPIEnabled() {
					adCfg := cfg.GetAntiDPIConfig()
					antiDPIServer = NewAntiDPIServer(adCfg)
					if err := antiDPIServer.Start(ctx); err != nil {
						log.Printf("[antidpi] Внимание: не удалось запустить Anti-DPI сервер: %v", err)
					} else {
						_ = WriteDnsmasqAntiDPI(adCfg.GetTargetDomains())
						needDnsmasqRestart = true
					}
				}
				if cfg.IsForceVPNEnabled() {
					fvOpts := cfg.GetForceVPNOptions()
					fvDomains := cfg.ForceVPN.ResolveTargetDomains()
					if len(fvDomains) > 0 {
						_ = SetupDnsmasqForceVPN(fvDomains, fvOpts.DNSServer, false)
						forceVPNActive = true
						needDnsmasqRestart = true
					}
					log.Printf("[forcevpn] Активирован принудительный VPN (доменов: %d, статических CIDR: %d, DNS: %s)",
						len(fvDomains), len(fvOpts.StaticCIDRs), fvOpts.DNSServer)
					if needDnsmasqRestart {
						restartDnsmasq()
						needDnsmasqRestart = false
					}
					go warmForceVPNDomains(ctx, fvDomains)
				}
				if needDnsmasqRestart {
					restartDnsmasq()
				}
			case "log":
				// Логи выводятся в реальном времени через desktopCfg.LogFunc
			case "stopping":
				log.Printf("[engine] Остановка: %s", ev.Reason)
			case "stopped":
				log.Printf("[engine] Завершено: %s", ev.Reason)
				stopDnsExtensions()
				cleanupRouting(cfg, tun, serverHosts, wan)
				close(doneCh)
				return
			}
		}
	}()

	select {
	case sig := <-sigCh:
		log.Printf("Получен сигнал %s, завершение работы...", sig)
		stopDnsExtensions()
		cleanupRouting(cfg, tun, serverHosts, wan)
		handle.Stop()
		cancel()
		tun.Close()
	case <-doneCh:
		log.Println("Сессия туннеля завершена.")
	}
}
