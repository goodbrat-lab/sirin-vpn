package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
)

type mockReleaseProvider struct {
	versions map[string]string
	urls     map[string]string
}

func (m *mockReleaseProvider) GetLatestVersion(platform string) (string, string) {
	return m.versions[platform], m.urls[platform]
}

func TestCheckKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bot_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "bot_users.json")
	storage, err := bot.NewStorage(storagePath)
	if err != nil {
		t.Fatal(err)
	}

	telegramID := int64(123456)
	now := time.Now()

	t.Run("nil client", func(t *testing.T) {
		res := CheckKey(nil, nil, telegramID, storage)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail, got %s", res.Status)
		}
	})

	t.Run("revoked client", func(t *testing.T) {
		c := &bot.Client{
			UUID:    "11111111-1111-1111-1111-111111111111",
			Revoked: true,
		}
		res := CheckKey(c, nil, telegramID, storage)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail, got %s", res.Status)
		}
	})

	t.Run("expired client", func(t *testing.T) {
		past := now.Add(-24 * time.Hour)
		c := &bot.Client{
			UUID:      "11111111-1111-1111-1111-111111111111",
			ExpiresAt: &past,
		}
		res := CheckKey(c, nil, telegramID, storage)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail, got %s", res.Status)
		}
	})

	t.Run("expiring soon (< 3 days)", func(t *testing.T) {
		soon := now.Add(48 * time.Hour)
		c := &bot.Client{
			UUID:      "11111111-1111-1111-1111-111111111111",
			ExpiresAt: &soon,
		}
		user := &bot.UserKey{TelegramID: telegramID, UUID: c.UUID}
		res := CheckKey(c, user, telegramID, storage)
		if res.Status != StatusWarn {
			t.Fatalf("expected StatusWarn, got %s", res.Status)
		}
	})

	t.Run("active client without telegram binding has autofix", func(t *testing.T) {
		future := now.Add(30 * 24 * time.Hour)
		c := &bot.Client{
			UUID:      "11111111-1111-1111-1111-111111111111",
			Name:      "TestUser",
			ExpiresAt: &future,
		}
		res := CheckKey(c, nil, telegramID, storage)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
		if res.AutoFix == nil || res.AutoFix.ID != "link_telegram" {
			t.Fatalf("expected link_telegram autofix, got %+v", res.AutoFix)
		}

		// Apply fix
		if err := res.AutoFix.Apply(context.Background()); err != nil {
			t.Fatalf("autofix apply failed: %v", err)
		}

		u, ok := storage.Get(telegramID)
		if !ok || u.UUID != c.UUID {
			t.Fatalf("expected user to be saved in storage")
		}
	})
}

func TestCheckAPI(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "ok",
				"clients":   8,
				"egress_ip": "94.183.159.114",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	t.Run("client view hides clients count", func(t *testing.T) {
		res, egressIP := CheckAPI(context.Background(), srv.Client(), srv.URL, false)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
		if egressIP != "94.183.159.114" {
			t.Fatalf("expected egress IP 94.183.159.114, got %s", egressIP)
		}
		if res.Details != "" {
			t.Fatalf("expected empty details for client view, got %s", res.Details)
		}
	})

	t.Run("admin view shows clients count", func(t *testing.T) {
		res, _ := CheckAPI(context.Background(), srv.Client(), srv.URL, true)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
		if res.Details == "" {
			t.Fatalf("expected non-empty details for admin view")
		}
	})

	t.Run("server error", func(t *testing.T) {
		res, _ := CheckAPI(context.Background(), srv.Client(), "http://127.0.0.1:59999", false)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail, got %s", res.Status)
		}
	})
}

func TestCheckParams(t *testing.T) {
	c := &bot.Client{UUID: "11111111-1111-1111-1111-111111111111"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == c.UUID {
			w.Header().Set("X-Relay-Load", "3/10")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"contract": "v1",
				"status":   "answered",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"contract": "v1",
			"status":   "refused",
			"error":    "ключ не найден",
		})
	}))
	defer srv.Close()

	t.Run("valid key", func(t *testing.T) {
		res, header := CheckParams(context.Background(), srv.Client(), srv.URL, c)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
		if header != "3/10" {
			t.Fatalf("expected 3/10, got %s", header)
		}
	})

	t.Run("refused key", func(t *testing.T) {
		badClient := &bot.Client{UUID: "00000000-0000-0000-0000-000000000000"}
		res, _ := CheckParams(context.Background(), srv.Client(), srv.URL, badClient)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail, got %s", res.Status)
		}
	})
}

func TestCheckRelay(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "relay_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	relaysPath := filepath.Join(tmpDir, "relays.json")
	clientsPath := filepath.Join(tmpDir, "clients.json")

	rStorage, err := relays.NewStorage(relaysPath)
	if err != nil {
		t.Fatal(err)
	}

	cStorage, err := bot.NewClientsStorage(clientsPath)
	if err != nil {
		t.Fatal(err)
	}

	client, err := cStorage.AddWithRelay("TestRelayUser", 30, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("relay direct NL", func(t *testing.T) {
		nlClient := &bot.Client{RelayID: 0}
		res := CheckRelay(context.Background(), nlClient, rStorage, cStorage, nil)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
	})

	t.Run("relay active with probe", func(t *testing.T) {
		probe := func(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
			return 45 * time.Millisecond, nil
		}
		res := CheckRelay(context.Background(), client, rStorage, cStorage, probe)
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK, got %s", res.Status)
		}
	})

	t.Run("relay probe fail gives warn without autofix", func(t *testing.T) {
		probe := func(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
			return 0, errors.New("timeout")
		}
		res := CheckRelay(context.Background(), client, rStorage, cStorage, probe)
		if res.Status != StatusWarn {
			t.Fatalf("expected StatusWarn, got %s", res.Status)
		}
		if res.AutoFix != nil {
			t.Fatalf("expected no autofix for probe timeout, got %+v", res.AutoFix)
		}
	})

	t.Run("relay probe high RTT gives warn", func(t *testing.T) {
		probe := func(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
			return 200 * time.Millisecond, nil
		}
		res := CheckRelay(context.Background(), client, rStorage, cStorage, probe)
		if res.Status != StatusWarn {
			t.Fatalf("expected StatusWarn for high RTT, got %s", res.Status)
		}
		if res.AutoFix != nil {
			t.Fatalf("expected no autofix for high RTT, got %+v", res.AutoFix)
		}
	})

	t.Run("disabled relay has autofix", func(t *testing.T) {
		// Toggle relay to disabled
		_, _ = rStorage.Toggle(1)
		res := CheckRelay(context.Background(), client, rStorage, cStorage, nil)
		if res.Status != StatusFail {
			t.Fatalf("expected StatusFail for disabled relay, got %s", res.Status)
		}
		if res.AutoFix == nil || res.AutoFix.ID != "reset_relay_nl" {
			t.Fatalf("expected reset_relay_nl autofix, got %+v", res.AutoFix)
		}

		// Apply fix
		if err := res.AutoFix.Apply(context.Background()); err != nil {
			t.Fatalf("failed to apply autofix: %v", err)
		}
		updated, _ := cStorage.Get(client.UUID)
		if updated.RelayID != 0 {
			t.Fatalf("expected RelayID 0, got %d", updated.RelayID)
		}
	})
}

func TestCheckClientContext(t *testing.T) {
	rel := &mockReleaseProvider{
		versions: map[string]string{"android": "1.3.5"},
		urls:     map[string]string{"android": "https://example.com/download"},
	}

	t.Run("matching IP", func(t *testing.T) {
		input := ClientInput{ClientIP: "94.183.159.114"}
		res := CheckClientContext(input, "94.183.159.114", rel)
		if len(res) == 0 || res[0].Status != StatusOK {
			t.Fatalf("expected StatusOK for matching IP, got %+v", res)
		}
	})

	t.Run("mismatched IP", func(t *testing.T) {
		input := ClientInput{ClientIP: "1.2.3.4"}
		res := CheckClientContext(input, "94.183.159.114", rel)
		if len(res) == 0 || res[0].Status != StatusWarn {
			t.Fatalf("expected StatusWarn for mismatched IP, got %+v", res)
		}
	})

	t.Run("outdated version has update autofix", func(t *testing.T) {
		input := ClientInput{Platform: "android", AppVersion: "1.3.4"}
		res := CheckClientContext(input, "", rel)
		if len(res) == 0 || res[0].Status != StatusWarn {
			t.Fatalf("expected StatusWarn for outdated version, got %+v", res)
		}
		if res[0].AutoFix == nil || res[0].AutoFix.ID != "update_app" {
			t.Fatalf("expected update_app autofix, got %+v", res[0].AutoFix)
		}
	})
}

func TestCheckTransport(t *testing.T) {
	t.Run("udp warns about mobile carrier throttling", func(t *testing.T) {
		res := CheckTransport(ClientInput{Transport: "udp"})
		if res.Status != StatusWarn {
			t.Fatalf("expected StatusWarn for UDP, got %s", res.Status)
		}
	})

	t.Run("vless is ok", func(t *testing.T) {
		res := CheckTransport(ClientInput{Transport: "vless"})
		if res.Status != StatusOK {
			t.Fatalf("expected StatusOK for vless, got %s", res.Status)
		}
	})
}

func TestEngine_Run(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "ok",
				"clients":   5,
				"egress_ip": "94.183.159.114",
			})
			return
		}
		if r.URL.Path == "/v1/params" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"contract": "v1",
				"status":   "answered",
			})
			return
		}
	}))
	defer srv.Close()

	rel := &mockReleaseProvider{
		versions: map[string]string{"android": "1.3.5"},
		urls:     map[string]string{"android": "https://example.com"},
	}

	deps := Dependencies{
		APIURL:     srv.URL,
		HTTPClient: srv.Client(),
		Releases:   rel,
		TCPProbe: func(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
			return 20 * time.Millisecond, nil
		},
	}

	engine := NewEngine(deps)

	client := &bot.Client{
		UUID:    "11111111-1111-1111-1111-111111111111",
		RelayID: 0,
	}
	user := &bot.UserKey{
		TelegramID: 12345,
		UUID:       client.UUID,
	}
	input := ClientInput{
		Platform:   "android",
		AppVersion: "1.3.5",
		Transport:  "vless",
		ClientIP:   "94.183.159.114",
	}

	report := engine.Run(context.Background(), client, user, 12345, input)
	if report.Overall != StatusOK {
		t.Fatalf("expected Overall StatusOK, got %s (%+v)", report.Overall, report.Checks)
	}

	text := report.FormatText(false)
	if len(text) == 0 {
		t.Fatalf("expected formatted text")
	}
}
