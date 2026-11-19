package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func keyVerdictFor(t *testing.T, srv *Server, uuid, device string) KeyVerdict {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/key?key="+uuid+"&device_id="+device, nil)
	w := httptest.NewRecorder()
	srv.HandleKey(w, req)
	var v KeyVerdict
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return v
}

func TestMultiDeviceLimit(t *testing.T) {
	srv, storage := testServer(t)
	c, _ := storage.Add("Family", 0)
	if err := storage.SetMaxDevices(c.UUID, 2); err != nil {
		t.Fatalf("SetMaxDevices: %v", err)
	}

	if v := keyVerdictFor(t, srv, c.UUID, "phone"); v.Bound != "this" {
		t.Fatalf("phone: expected bound=this, got %q", v.Bound)
	}
	if v := keyVerdictFor(t, srv, c.UUID, "router"); v.Bound != "this" {
		t.Fatalf("router: expected bound=this, got %q", v.Bound)
	}
	// третье устройство сверх лимита 2
	if v := keyVerdictFor(t, srv, c.UUID, "laptop"); v.Bound != "other" {
		t.Fatalf("laptop: expected bound=other (limit reached), got %q", v.Bound)
	}
	// уже привязанное устройство по-прежнему допускается
	if v := keyVerdictFor(t, srv, c.UUID, "phone"); v.Bound != "this" {
		t.Fatalf("phone again: expected bound=this, got %q", v.Bound)
	}

	// после отвязки роутера место освобождается
	if err := storage.UnbindDevice(c.UUID, "router"); err != nil {
		t.Fatalf("UnbindDevice: %v", err)
	}
	if v := keyVerdictFor(t, srv, c.UUID, "laptop"); v.Bound != "this" {
		t.Fatalf("laptop after unbind: expected bound=this, got %q", v.Bound)
	}
}

func TestMultiDeviceUnlimitedByDefault(t *testing.T) {
	srv, storage := testServer(t)
	c, _ := storage.Add("Solo", 0)
	for _, d := range []string{"a", "b", "c", "d"} {
		if v := keyVerdictFor(t, srv, c.UUID, d); v.Bound != "this" {
			t.Fatalf("device %s: expected bound=this without limit, got %q", d, v.Bound)
		}
	}
}

func TestMultiDevicePersistsAndLegacyDeviceID(t *testing.T) {
	_, storage := testServer(t)
	c, _ := storage.Add("Legacy", 0)
	_ = storage.Touch(c.UUID, "old-dev") // старый путь: заполняет только DeviceID
	_ = storage.SetMaxDevices(c.UUID, 1)

	if ok, _ := storage.TouchDevice(c.UUID, "old-dev"); !ok {
		t.Fatal("legacy device must remain allowed")
	}
	if ok, _ := storage.TouchDevice(c.UUID, "new-dev"); ok {
		t.Fatal("second device must be rejected with max_devices=1")
	}
	got, _ := storage.Get(c.UUID)
	if got.MaxDevices != 1 || len(got.DeviceIDs) != 1 || got.DeviceIDs[0] != "old-dev" {
		t.Fatalf("unexpected persisted state: %+v", got)
	}
}
