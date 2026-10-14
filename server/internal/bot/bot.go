package bot

import (
	"log"
	"time"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	cfg     *Config
	bot     *tele.Bot
	storage *Storage
	api     *APIClient
}

func New(cfg *Config, storage *Storage) (*Bot, error) {
	prefs := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	tb, err := tele.NewBot(prefs)
	if err != nil {
		return nil, err
	}
	b := &Bot{
		cfg:     cfg,
		bot:     tb,
		storage: storage,
		api:     NewAPIClient(cfg.APIURL, cfg.APIAdminToken),
	}
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	log.Printf("[SirinBot] starting as @%s", b.bot.Me.Username)
	b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) registerHandlers() {
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/newkey", b.handleNewKey)
	b.bot.Handle("/mykey", b.handleMyKey)
	b.bot.Handle("/status", b.handleStatus)
}

func (b *Bot) isAdmin(id int64) bool {
	return id == b.cfg.AdminID
}
