package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"sirin/server/internal/config"
	"sirin/server/internal/noise"
	"sirin/server/internal/server"
)

func main() {
	configPath := flag.String("config", "configs/sirind.toml", "Путь к файлу конфигурации")
	genKey := flag.Bool("genkey", false, "Сгенерировать новую пару статических ключей X25519")
	flag.Parse()

	if *genKey {
		if err := generateKeypair(); err != nil {
			log.Fatalf("Ошибка генерации ключей: %v", err)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	staticPriv, err := loadStaticKey(cfg.StaticPrivFile)
	if err != nil {
		log.Fatalf("Ошибка загрузки статического ключа сервера: %v", err)
	}

	srv, err := server.New(cfg, staticPriv)
	if err != nil {
		log.Fatalf("Ошибка создания сервера: %v", err)
	}

	if srv.TunEnabled() {
		log.Printf("TUN device %s configured: %s/24", cfg.TunName, cfg.TunIP)
	} else {
		log.Println("TUN disabled (dev mode)")
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Получен сигнал завершения работы, останавливаем сервер...")
	srv.Stop()
}

func generateKeypair() error {
	priv, pub, err := noise.GenerateKeyPair()
	if err != nil {
		return err
	}
	fmt.Printf("Private Key (hex, 32 bytes): %s\n", hex.EncodeToString(priv))
	fmt.Printf("Public Key  (hex, 32 bytes): %s\n", hex.EncodeToString(pub))
	return nil
}

func loadStaticKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := strings.TrimSpace(string(data))
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("неверный hex в файле ключа: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ожидалось 32 байта ключа, получено %d", len(key))
	}
	return key, nil
}
