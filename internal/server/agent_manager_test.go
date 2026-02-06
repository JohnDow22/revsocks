package server

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ========================================
// Unit Tests для AgentManager
// ========================================

// TestNewAgentManager проверяет создание нового AgentManager
func TestNewAgentManager(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")

	am, err := NewAgentManager(tempFile)
	if err != nil {
		t.Fatalf("Failed to create AgentManager: %v", err)
	}

	if am == nil {
		t.Fatal("AgentManager is nil")
	}

	if am.dbPath != tempFile {
		t.Errorf("Expected dbPath=%s, got %s", tempFile, am.dbPath)
	}
}

// TestRegisterAgent_NewAgent проверяет регистрацию нового агента
func TestRegisterAgent_NewAgent(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	agentID := "test-agent-1"
	ip := "192.168.1.100"
	version := "v3"

	agent, err := am.RegisterAgent(agentID, ip, version)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Проверяем дефолтные значения
	if agent.ID != agentID {
		t.Errorf("Expected ID=%s, got %s", agentID, agent.ID)
	}

	if agent.IP != ip {
		t.Errorf("Expected IP=%s, got %s", ip, agent.IP)
	}

	if agent.Mode != StateTunnel {
		t.Errorf("Expected Mode=%s, got %s", StateTunnel, agent.Mode)
	}

	if agent.SleepInterval != 60 {
		t.Errorf("Expected SleepInterval=60, got %d", agent.SleepInterval)
	}

	if agent.Jitter != 10 {
		t.Errorf("Expected Jitter=10, got %d", agent.Jitter)
	}

	if agent.FirstSeen.IsZero() {
		t.Error("FirstSeen should not be zero")
	}

	if agent.LastSeen.IsZero() {
		t.Error("LastSeen should not be zero")
	}
}

// TestRegisterAgent_ExistingAgent проверяет обновление существующего агента
func TestRegisterAgent_ExistingAgent(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	agentID := "test-agent-1"
	ip1 := "192.168.1.100"
	ip2 := "192.168.1.200"

	// Первая регистрация
	agent1, _ := am.RegisterAgent(agentID, ip1, "v3")
	firstSeen := agent1.FirstSeen
	lastSeen1 := agent1.LastSeen

	// Ждём чтобы LastSeen изменился
	time.Sleep(10 * time.Millisecond)

	// Вторая регистрация (имитация reconnect)
	agent2, _ := am.RegisterAgent(agentID, ip2, "v3")

	// FirstSeen НЕ должен измениться
	if !agent2.FirstSeen.Equal(firstSeen) {
		t.Errorf("FirstSeen changed after reconnect: was %v, now %v", firstSeen, agent2.FirstSeen)
	}

	// LastSeen ДОЛЖЕН измениться
	if !agent2.LastSeen.After(lastSeen1) {
		t.Error("LastSeen should be updated on reconnect")
	}

	// IP должен обновиться
	if agent2.IP != ip2 {
		t.Errorf("Expected IP=%s, got %s", ip2, agent2.IP)
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// TestUpdateState проверяет обновление состояния агента
func TestUpdateState(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)
	
	// Cleanup: ждём завершения всех async операций
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
	})

	agentID := "test-agent-1"
	am.RegisterAgent(agentID, "192.168.1.100", "v3")

	// Обновляем состояние на SLEEP
	err := am.UpdateState(agentID, StateSleep, 120, 20)
	if err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	// Проверяем что изменения применились
	agent := am.GetConfig(agentID)
	if agent.Mode != StateSleep {
		t.Errorf("Expected Mode=%s, got %s", StateSleep, agent.Mode)
	}

	if agent.SleepInterval != 120 {
		t.Errorf("Expected SleepInterval=120, got %d", agent.SleepInterval)
	}

	if agent.Jitter != 20 {
		t.Errorf("Expected Jitter=20, got %d", agent.Jitter)
	}
}

// TestUpdateState_NotFound проверяет ошибку для несуществующего агента
func TestUpdateState_NotFound(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	err := am.UpdateState("non-existent-agent", StateSleep, 60, 10)
	if err == nil {
		t.Error("Expected error for non-existent agent, got nil")
	}
}

// TestUpdateAlias проверяет обновление алиаса
func TestUpdateAlias(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)
	
	// Cleanup: ждём завершения всех async операций
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
	})

	agentID := "test-agent-1"
	am.RegisterAgent(agentID, "192.168.1.100", "v3")

	newAlias := "Production Server"
	err := am.UpdateAlias(agentID, newAlias)
	if err != nil {
		t.Fatalf("Failed to update alias: %v", err)
	}

	agent := am.GetConfig(agentID)
	if agent.Alias != newAlias {
		t.Errorf("Expected Alias=%s, got %s", newAlias, agent.Alias)
	}
}

// TestSaveLoad проверяет сохранение и загрузку из JSON
func TestSaveLoad(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")

	// Создаём менеджер и регистрируем агентов
	am1, _ := NewAgentManager(tempFile)
	am1.RegisterAgent("agent-1", "192.168.1.1", "v3")
	am1.RegisterAgent("agent-2", "192.168.1.2", "v3")
	am1.UpdateState("agent-1", StateSleep, 90, 15)

	// Явно сохраняем (RegisterAgent делает это асинхронно)
	time.Sleep(50 * time.Millisecond)
	if err := am1.Save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Создаём новый менеджер и загружаем из файла
	am2, err := NewAgentManager(tempFile)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Проверяем что данные загрузились
	agents := am2.ListAgents()
	if len(agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(agents))
	}

	// Проверяем конкретного агента
	agent1 := am2.GetConfig("agent-1")
	if agent1 == nil {
		t.Fatal("Agent-1 not found after load")
	}

	if agent1.Mode != StateSleep {
		t.Errorf("Expected Mode=%s, got %s", StateSleep, agent1.Mode)
	}

	if agent1.SleepInterval != 90 {
		t.Errorf("Expected SleepInterval=90, got %d", agent1.SleepInterval)
	}
}

// TestListAgents проверяет получение списка всех агентов
func TestListAgents(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)
	
	// Cleanup: ждём завершения всех async операций
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
	})

	// Пустой список
	agents := am.ListAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(agents))
	}

	// Добавляем агентов
	am.RegisterAgent("agent-1", "192.168.1.1", "v3")
	am.RegisterAgent("agent-2", "192.168.1.2", "v3")
	am.RegisterAgent("agent-3", "192.168.1.3", "v2")

	agents = am.ListAgents()
	if len(agents) != 3 {
		t.Fatalf("Expected 3 agents, got %d", len(agents))
	}

	// Проверяем что это копии (изменение не влияет на оригинал)
	agents[0].Mode = StateSleep
	original := am.GetConfig("agent-1")
	if original.Mode == StateSleep {
		t.Error("ListAgents should return copies, not references")
	}
}

// TestDeleteAgent проверяет удаление агента
func TestDeleteAgent(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	agentID := "test-agent-1"
	am.RegisterAgent(agentID, "192.168.1.100", "v3")

	// Проверяем что агент существует
	if am.GetConfig(agentID) == nil {
		t.Fatal("Agent should exist before deletion")
	}

	// Удаляем
	err := am.DeleteAgent(agentID)
	if err != nil {
		t.Fatalf("Failed to delete agent: %v", err)
	}

	// Проверяем что агента больше нет
	if am.GetConfig(agentID) != nil {
		t.Error("Agent should be deleted")
	}

	// Повторное удаление должно вернуть ошибку
	err = am.DeleteAgent(agentID)
	if err == nil {
		t.Error("Expected error when deleting non-existent agent")
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// TestThreadSafety проверяет потокобезопасность RegisterAgent
func TestThreadSafety(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	// Запускаем 100 горутин одновременно
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := "agent-1" // Все горутины регистрируют одного агента
			ip := "192.168.1.100"
			_, err := am.RegisterAgent(agentID, ip, "v3")
			if err != nil {
				t.Errorf("RegisterAgent failed in goroutine %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем что агент зарегистрирован корректно
	agent := am.GetConfig("agent-1")
	if agent == nil {
		t.Fatal("Agent should be registered after concurrent calls")
	}
	
	// Ждём async Save() от всех горутин
	time.Sleep(200 * time.Millisecond)
}

// TestGetConfig_NotFound проверяет что GetConfig возвращает nil для несуществующего агента
func TestGetConfig_NotFound(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, _ := NewAgentManager(tempFile)

	agent := am.GetConfig("non-existent-agent")
	if agent != nil {
		t.Error("Expected nil for non-existent agent")
	}
}

// TestLoadInvalidJSON проверяет обработку невалидного JSON
func TestLoadInvalidJSON(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "invalid.json")

	// Пишем невалидный JSON
	os.WriteFile(tempFile, []byte("invalid json {{{"), 0600)

	am, err := NewAgentManager(tempFile)
	if err != nil {
		t.Fatalf("NewAgentManager should not fail, got: %v", err)
	}

	// AgentManager должен быть создан (Load возвращает warning, но не ошибку)
	if am == nil {
		t.Fatal("AgentManager should be created even with invalid JSON")
	}
}
