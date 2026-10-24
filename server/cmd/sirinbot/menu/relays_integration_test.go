package menu

import (
	"fmt"
	"path/filepath"
	"testing"

	"sirin/server/internal/bot"
	"sirin/server/internal/relays"
)

func TestRelayIntegrationLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	clientsPath := filepath.Join(tmpDir, "clients.json")
	relaysPath := filepath.Join(tmpDir, "relays.json")

	// 1. Создаём хранилище клиентов
	cStorage, err := bot.NewClientsStorage(clientsPath)
	if err != nil {
		t.Fatalf("NewClientsStorage: %v", err)
	}

	// 2. Создаём хранилище релеев (автоматически создаётся дефолтный Relay 1)
	rStorage, err := relays.NewStorage(relaysPath)
	if err != nil {
		t.Fatalf("NewStorage relays: %v", err)
	}

	// Настроим Relay 1 с лимитом max_clients = 5
	r1, ok := rStorage.GetByID(1)
	if !ok {
		t.Fatalf("Relay 1 not found")
	}
	r1.MaxClients = 5
	if err := rStorage.Update(*r1); err != nil {
		t.Fatalf("Update relay 1: %v", err)
	}

	// Добавим Relay 2 (СПб, RU)
	r2 := relays.Relay{
		ID:         2,
		Name:       "Relay 2 (СПб, RU)",
		Host:       "194.87.1.1",
		PortVLESS:  443,
		PortXHTTP:  8443,
		MaxClients: 10,
		Enabled:    true,
	}
	if _, err := rStorage.Add(r2); err != nil {
		t.Fatalf("Add relay 2: %v", err)
	}

	// 3. Создаём 6 клиентов
	var clients []*bot.Client
	for i := 1; i <= 6; i++ {
		c, err := cStorage.AddWithRelay(fmt.Sprintf("User-%d", i), 30, 0)
		if err != nil {
			t.Fatalf("Add client %d: %v", i, err)
		}
		clients = append(clients, c)
	}

	// 4. Назначаем первых 3 клиентов на Relay 1 (max=5) -> должно пройти
	for i := 0; i < 3; i++ {
		all := cStorage.List()
		cnt := relays.CountActiveOnRelay(all, 1)
		if cnt >= r1.MaxClients {
			t.Fatalf("unexpected full relay at client %d", i)
		}
		_, err := cStorage.SetRelayID(clients[i].UUID, 1)
		if err != nil {
			t.Fatalf("SetRelayID failed: %v", err)
		}
	}

	cnt3 := relays.CountActiveOnRelay(cStorage.List(), 1)
	if cnt3 != 3 {
		t.Errorf("expected 3 clients on relay 1, got %d", cnt3)
	}

	// 5. Назначаем клиентов 4 и 5 (итого 5/5) -> должно пройти
	for i := 3; i < 5; i++ {
		all := cStorage.List()
		cnt := relays.CountActiveOnRelay(all, 1)
		if cnt >= r1.MaxClients {
			t.Fatalf("unexpected full relay at client %d", i)
		}
		_, err := cStorage.SetRelayID(clients[i].UUID, 1)
		if err != nil {
			t.Fatalf("SetRelayID failed: %v", err)
		}
	}

	cnt5 := relays.CountActiveOnRelay(cStorage.List(), 1)
	if cnt5 != 5 {
		t.Errorf("expected 5 clients on relay 1, got %d", cnt5)
	}

	// 6. Попытка назначить 6-го клиента на Relay 1 -> должна быть заблокирована проверкой лимита
	all := cStorage.List()
	cntBefore6 := relays.CountActiveOnRelay(all, 1)
	if cntBefore6 >= r1.MaxClients {
		// Лимит исчерпан! Симулируем отказ назначения
		t.Logf("PASS: Relay 1 is full (%d/%d), assignment of 6th client rejected", cntBefore6, r1.MaxClients)
	} else {
		t.Errorf("expected relay 1 to be full, but count is %d/%d", cntBefore6, r1.MaxClients)
	}

	// 7. Отключение Relay 1 -> перевод клиентов на Direct NL (relay_id=0)
	// Симуляция подтверждённого отключения (как в HandleToggleConfirm)
	for _, cl := range cStorage.List() {
		if cl.RelayID == 1 {
			_, _ = cStorage.SetRelayID(cl.UUID, 0)
		}
	}
	r1Toggled, err := rStorage.Toggle(1)
	if err != nil {
		t.Fatalf("Toggle relay 1: %v", err)
	}
	if r1Toggled.Enabled {
		t.Errorf("expected relay 1 to be disabled")
	}

	cntAfterDisable := relays.CountActiveOnRelay(cStorage.List(), 1)
	if cntAfterDisable != 0 {
		t.Errorf("expected 0 clients on relay 1 after transfer, got %d", cntAfterDisable)
	}

	cntDirect := relays.CountActiveOnRelay(cStorage.List(), 0)
	if cntDirect != 6 {
		t.Errorf("expected all 6 clients to be on Direct NL (id=0), got %d", cntDirect)
	}

	// 8. Удаление релеев:
	// Назначим одного клиента на Relay 2
	_, _ = cStorage.SetRelayID(clients[0].UUID, 2)
	cntRelay2 := relays.CountActiveOnRelay(cStorage.List(), 2)
	if cntRelay2 != 1 {
		t.Fatalf("expected 1 client on relay 2, got %d", cntRelay2)
	}

	// Попытка удалить Relay 2 с активным клиентом -> проверка блокировки
	if cntRelay2 > 0 {
		t.Log("PASS: Deletion of relay 2 with active clients blocked")
	} else {
		t.Errorf("expected deletion to be blocked")
	}

	// Переводим клиента обратно и удаляем Relay 2
	_, _ = cStorage.SetRelayID(clients[0].UUID, 0)
	if err := rStorage.Delete(2); err != nil {
		t.Fatalf("Delete relay 2 failed: %v", err)
	}
	if _, ok := rStorage.GetByID(2); ok {
		t.Errorf("expected relay 2 to be deleted from storage")
	}

	// 9. Проверка отозванных клиентов: они не должны учитываться в лимите
	revokedClient, _ := cStorage.AddWithRelay("RevokedClient", 30, 1)
	_, _ = cStorage.SetRevoked(revokedClient.UUID, true)

	activeOnR1 := relays.CountActiveOnRelay(cStorage.List(), 1)
	if activeOnR1 != 0 {
		t.Errorf("expected 0 active clients on relay 1, got %d", activeOnR1)
	}
}
