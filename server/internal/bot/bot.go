package bot

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"sirin/server/internal/relays"
	"sirin/server/internal/bot/tickets"
)

type pendingDelete struct {
	UUID      string
	ExpiresAt time.Time
}

type Bot struct {
	cfg            *Config
	bot            *tele.Bot
	storage        *Storage        // bot_users.json (telegram_id -> UserKey)
	clientsStorage *ClientsStorage // clients.json
	relaysStorage  *relays.Storage // relays.json
	ticketsStorage *tickets.Storage // tickets.json
	api            *APIClient
	pendingDeletes map[string]pendingDelete
	pendingDelMu   sync.Mutex
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

	clientsStorage, err := NewClientsStorage(cfg.ClientsPath)
	if err != nil {
		return nil, err
	}

	var relaysStorage *relays.Storage
	if cfg.RelaysPath != "" {
		relaysStorage, err = relays.NewStorage(cfg.RelaysPath)
		if err != nil {
			log.Printf("[SirinBot] relays storage init error (%s): %v", cfg.RelaysPath, err)
			return nil, err
		}
	}

	var ticketsStorage *tickets.Storage
	if cfg.TicketsPath != "" {
		var err error
		ticketsStorage, err = tickets.LoadTickets(cfg.TicketsPath)
		if err != nil {
			log.Printf("[SirinBot] tickets storage init error (%s): %v", cfg.TicketsPath, err)
		}
	}

	b := &Bot{
		cfg:            cfg,
		bot:            tb,
		storage:        storage,
		clientsStorage: clientsStorage,
		relaysStorage:  relaysStorage,
		ticketsStorage: ticketsStorage,
		api:            NewAPIClient(cfg.APIURL, cfg.APIAdminToken),
		pendingDeletes: make(map[string]pendingDelete),
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

func (b *Bot) TeleBot() *tele.Bot { return b.bot }
func (b *Bot) ClientsStorage() *ClientsStorage { return b.clientsStorage }
func (b *Bot) RelaysStorage() *relays.Storage { return b.relaysStorage }
func (b *Bot) SetRelaysStorage(rs *relays.Storage) { b.relaysStorage = rs }
func (b *Bot) TicketsStorage() *tickets.Storage { return b.ticketsStorage }
func (b *Bot) SetTicketsStorage(ts *tickets.Storage) { b.ticketsStorage = ts }
func (b *Bot) Storage() *Storage { return b.storage }
func (b *Bot) Config() *Config { return b.cfg }
func (b *Bot) IsAdmin(id int64) bool { return b.isAdmin(id) }
func (b *Bot) AdminOnly(next tele.HandlerFunc) tele.HandlerFunc { return b.adminOnly(next) }
func (b *Bot) LogAdminAction(adminID int64, action, clientPrefix string, ok bool) {
	b.logAdminAction(adminID, action, clientPrefix, ok)
}

func (b *Bot) isAdmin(id int64) bool {
	return b.cfg.IsAdmin(id)
}

func (b *Bot) adminOnly(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if !b.isAdmin(c.Sender().ID) {
			b.logAdminAction(c.Sender().ID, c.Text(), "", false)
			return c.Send("⛔ Эта команда доступна только администратору.")
		}
		return next(c)
	}
}

func (b *Bot) logAdminAction(adminID int64, action, clientPrefix string, ok bool) {
	if len(clientPrefix) > 8 {
		clientPrefix = clientPrefix[:8]
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"event":         "admin_action",
		"admin_id":      adminID,
		"action":        action,
		"client_prefix": clientPrefix,
		"ok":            ok,
	})
	log.Println(string(payload))
}

func (b *Bot) registerHandlers() {
	// Публичные команды
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/mykey", b.handleMyKey)

	// Админ-команды
	b.bot.Handle("/newkey", b.adminOnly(b.handleNewKey))
	b.bot.Handle("/status", b.adminOnly(b.handleStatus))
	b.bot.Handle("/list", b.adminOnly(b.handleList))
	b.bot.Handle("/info", b.adminOnly(b.handleInfo))
	b.bot.Handle("/rename", b.adminOnly(b.handleRename))
	b.bot.Handle("/extend", b.adminOnly(b.handleExtend))
	b.bot.Handle("/shorten", b.adminOnly(b.handleShorten))
	b.bot.Handle("/disable", b.adminOnly(b.handleDisable))
	b.bot.Handle("/enable", b.adminOnly(b.handleEnable))
	b.bot.Handle("/delete", b.adminOnly(b.handleDelete))
	b.bot.Handle("/find", b.adminOnly(b.handleFind))

	// Обработка inline-кнопок
	b.bot.Handle(tele.OnCallback, b.handleCallback)
}
