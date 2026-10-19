package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sirin/server/cmd/sirinbot/menu"
	"sirin/server/internal/bot"
)

func main() {
	cfgPath := flag.String("config", "configs/sirinbot.toml", "path to config")
	flag.Parse()

	cfg, err := bot.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validate: %v", err)
	}

	storage, err := bot.NewStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	b, err := bot.New(cfg, storage)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	menu.Register(b)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutdown signal received")
		b.Stop()
	}()

	b.Start()
}
