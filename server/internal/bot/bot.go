package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"

	"sirin/server/internal/relays"
	"sirin/server/internal/bot/ai"
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
	aiClient       *ai.Client
	expiryNotifier *ExpiryNotifier
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
		aiClient:       ai.NewClient(cfg.GeminiAPIKey),
		pendingDeletes: make(map[string]pendingDelete),
	}
	b.expiryNotifier = NewExpiryNotifier(b, 1*time.Hour)
	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	log.Printf("[SirinBot] starting as @%s", b.bot.Me.Username)
	if b.expiryNotifier != nil {
		b.expiryNotifier.Start()
	}
	b.bot.Start()
}

func (b *Bot) Stop() {
	if b.expiryNotifier != nil {
		b.expiryNotifier.Stop()
	}
	b.bot.Stop()
}

func (b *Bot) TeleBot() *tele.Bot { return b.bot }
func (b *Bot) ExpiryNotifier() *ExpiryNotifier { return b.expiryNotifier }
func (b *Bot) ClientsStorage() *ClientsStorage { return b.clientsStorage }
func (b *Bot) RelaysStorage() *relays.Storage { return b.relaysStorage }
func (b *Bot) SetRelaysStorage(rs *relays.Storage) { b.relaysStorage = rs }
func (b *Bot) TicketsStorage() *tickets.Storage { return b.ticketsStorage }
func (b *Bot) SetTicketsStorage(ts *tickets.Storage) { b.ticketsStorage = ts }
func (b *Bot) Storage() *Storage { return b.storage }
func (b *Bot) API() *APIClient { return b.api }
func (b *Bot) AI() *ai.Client { return b.aiClient }
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

// SendStarsInvoice отправляет инвойс на оплату подписки через Telegram Stars (XTR).
func (b *Bot) SendStarsInvoice(to tele.Recipient, tariff Tariff, clientUUID string) error {
	inv := &tele.Invoice{
		Title:       fmt.Sprintf("Sirin VPN — %s", tariff.Name),
		Description: fmt.Sprintf("Подписка Sirin VPN на %d дней. Мгновенная активация и поддержка всех устройств.", tariff.Days),
		Payload:     fmt.Sprintf("sirin_stars:%s:%s:%d:%d", clientUUID, tariff.ID, tariff.Days, time.Now().Unix()),
		Currency:    "XTR",
		Prices: []tele.Price{
			{Label: fmt.Sprintf("Sirin VPN (%s)", tariff.Name), Amount: tariff.Stars},
		},
		Token: "", // Для Telegram Stars провайдерский токен должен быть пустым
	}
	_, err := inv.Send(b.bot, to, nil)
	return err
}

func (b *Bot) registerHandlers() {
	// Публичные команды
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/mykey", b.handleMyKey)
	b.bot.Handle("/devices", b.handleDevices)
	b.bot.Handle("/tariffs", b.handleTariffs)

	// Telegram Stars платежи
	b.bot.Handle(tele.OnCheckout, b.handleCheckout)
	b.bot.Handle(tele.OnPayment, b.handlePayment)

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
	b.bot.Handle("/unbind", b.adminOnly(b.handleUnbind))
	b.bot.Handle("/maxdevices", b.adminOnly(b.handleMaxDevices))

	// Обработка inline-кнопок
	b.bot.Handle(tele.OnCallback, b.handleCallback)
}
