package relays

import (
	"path/filepath"
	"testing"
)

type dummyClient struct {
	relayID int
	active  bool
}

func (d dummyClient) GetRelayID() int { return d.relayID }
func (d dummyClient) IsActive() bool   { return d.active }

func TestRelaysInitDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "relays.json")

	s, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	all := s.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 default relay, got %d", len(all))
	}

	r := all[0]
	if r.ID != 1 || r.Host != "185.182.65.219" || !r.Enabled || r.MaxClients != 10 {
		t.Errorf("unexpected default relay values: %+v", r)
	}

	rByID, ok := s.GetByID(1)
	if !ok || rByID == nil || rByID.ID != 1 {
		t.Errorf("GetByID(1) failed")
	}

	next := s.NextID()
	if next != 2 {
		t.Errorf("expected NextID=2, got %d", next)
	}
}

func TestRelaysValidation(t *testing.T) {
	tests := []struct {
		name    string
		relay   Relay
		wantErr bool
	}{
		{
			name: "valid ip",
			relay: Relay{
				ID:         2,
				Name:       "Relay 2 (СПб, RU)",
				Host:       "194.87.1.1",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 20,
				Enabled:    true,
			},
			wantErr: false,
		},
		{
			name: "valid domain",
			relay: Relay{
				ID:         3,
				Name:       "Relay 3 (Екатеринбург)",
				Host:       "relay3.example.com",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 15,
				Enabled:    true,
			},
			wantErr: false,
		},
		{
			name: "invalid id 0",
			relay: Relay{
				ID:         0,
				Name:       "Relay 0",
				Host:       "1.1.1.1",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 10,
			},
			wantErr: true,
		},
		{
			name: "empty name",
			relay: Relay{
				ID:         2,
				Name:       "",
				Host:       "1.1.1.1",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 10,
			},
			wantErr: true,
		},
		{
			name: "invalid host with spaces",
			relay: Relay{
				ID:         2,
				Name:       "Relay 2",
				Host:       "1.1.1.1 ",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 10,
			},
			wantErr: true,
		},
		{
			name: "invalid port vless",
			relay: Relay{
				ID:         2,
				Name:       "Relay 2",
				Host:       "1.1.1.1",
				PortVLESS:  70000,
				PortXHTTP:  8443,
				MaxClients: 10,
			},
			wantErr: true,
		},
		{
			name: "invalid max_clients 0",
			relay: Relay{
				ID:         2,
				Name:       "Relay 2",
				Host:       "1.1.1.1",
				PortVLESS:  443,
				PortXHTTP:  8443,
				MaxClients: 0,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRelay(tc.relay)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateRelay() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRelaysCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "relays.json")

	s, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}

	// 1. Add Relay 2
	r2 := Relay{
		ID:         2,
		Name:       "Relay 2 (СПб)",
		Host:       "194.87.1.1",
		PortVLESS:  443,
		PortXHTTP:  8443,
		MaxClients: 15,
		Enabled:    false,
	}
	added, err := s.Add(r2)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if added.ID != 2 || added.Name != "Relay 2 (СПб)" {
		t.Errorf("unexpected added: %+v", added)
	}

	// Verify duplicate ID fails
	if _, err := s.Add(r2); err == nil {
		t.Errorf("expected error adding duplicate ID, got nil")
	}

	// 2. GetEnabled should only return Relay 1
	enabled := s.GetEnabled()
	if len(enabled) != 1 || enabled[0].ID != 1 {
		t.Fatalf("expected 1 enabled relay (id=1), got %d", len(enabled))
	}

	// 3. Toggle Relay 2
	toggled, err := s.Toggle(2)
	if err != nil {
		t.Fatalf("Toggle failed: %v", err)
	}
	if !toggled.Enabled {
		t.Errorf("expected relay 2 to be enabled")
	}
	if len(s.GetEnabled()) != 2 {
		t.Errorf("expected 2 enabled relays after toggle")
	}

	// 4. Update Relay 2
	toggled.MaxClients = 25
	if err := s.Update(*toggled); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, ok := s.GetByID(2)
	if !ok || updated.MaxClients != 25 {
		t.Errorf("expected max_clients 25, got %d", updated.MaxClients)
	}

	// 5. Delete Relay 2
	if err := s.Delete(2); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := s.GetByID(2); ok {
		t.Errorf("expected relay 2 to be deleted")
	}

	// 6. Persistence check: load in fresh instance
	s2, err := NewStorage(path)
	if err != nil {
		t.Fatalf("NewStorage s2 failed: %v", err)
	}
	if len(s2.GetAll()) != 1 {
		t.Fatalf("expected 1 relay restored, got %d", len(s2.GetAll()))
	}
}

func TestCountActiveOnRelay(t *testing.T) {
	clients := []dummyClient{
		{relayID: 1, active: true},
		{relayID: 1, active: true},
		{relayID: 1, active: false}, // expired or revoked
		{relayID: 0, active: true},
		{relayID: 2, active: true},
	}

	c1 := CountActiveOnRelay(clients, 1)
	if c1 != 2 {
		t.Errorf("expected 2 active clients on relay 1, got %d", c1)
	}

	c0 := CountActiveOnRelay(clients, 0)
	if c0 != 1 {
		t.Errorf("expected 1 active client on relay 0, got %d", c0)
	}

	c2 := CountActiveOnRelay(clients, 2)
	if c2 != 1 {
		t.Errorf("expected 1 active client on relay 2, got %d", c2)
	}

	c99 := CountActiveOnRelay(clients, 99)
	if c99 != 0 {
		t.Errorf("expected 0 active clients on relay 99, got %d", c99)
	}
}
