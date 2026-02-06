package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// ========================================
// Unit Tests для API
// ========================================

// setupTestAPI создаёт тестовый API server
func setupTestAPI(t *testing.T) (*AdminServer, *AgentManager) {
	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, err := NewAgentManager(tempFile)
	if err != nil {
		t.Fatalf("Failed to create AgentManager: %v", err)
	}

	srv := &AdminServer{
		manager:  am,
		sessions: GlobalSessionManager,
	}

	return srv, am
}

// setupTestAPIWithSessions создаёт API + изолированный SessionManager (без глобального состояния).
func setupTestAPIWithSessions(t *testing.T) (*AdminServer, *AgentManager, *SessionManager) {
	t.Helper()

	tempFile := filepath.Join(t.TempDir(), "test_agents.json")
	am, err := NewAgentManager(tempFile)
	if err != nil {
		t.Fatalf("Failed to create AgentManager: %v", err)
	}

	sm := NewSessionManager()

	srv := &AdminServer{
		manager:  am,
		sessions: sm,
	}

	return srv, am, sm
}

// TestAPIAuth_Valid проверяет успешную авторизацию
func TestAPIAuth_Valid(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Admin-Token", "test-token-12345")
	w := httptest.NewRecorder()

	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestAPIAuth_Invalid проверяет отказ при неверном токене
func TestAPIAuth_Invalid(t *testing.T) {
	srv, _ := setupTestAPI(t)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Admin-Token", "wrong-token")
	w := httptest.NewRecorder()

	// API без авторизации (localhost-only), всегда возвращает 200
	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (no auth required for localhost), got %d", w.Code)
	}
}

// TestAPIAuth_Missing проверяет отказ при отсутствии токена
func TestAPIAuth_Missing(t *testing.T) {
	srv, _ := setupTestAPI(t)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	// Не устанавливаем токен
	w := httptest.NewRecorder()

	// API без авторизации (localhost-only), всегда возвращает 200
	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (no auth required for localhost), got %d", w.Code)
	}
}

// TestListAgents_Empty проверяет пустой список агентов
func TestListAgents_Empty(t *testing.T) {
	srv, _ := setupTestAPI(t)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Admin-Token", "test-token-12345")
	w := httptest.NewRecorder()

	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var agents []AgentConfig
	if err := json.Unmarshal(w.Body.Bytes(), &agents); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(agents))
	}
}

// TestListAgents_Multiple проверяет список с несколькими агентами
func TestListAgents_Multiple(t *testing.T) {
	srv, am := setupTestAPI(t)

	am.RegisterAgent("agent-1", "192.168.1.1", "v3")
	am.RegisterAgent("agent-2", "192.168.1.2", "v3")
	am.RegisterAgent("agent-3", "192.168.1.3", "v2")

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("X-Admin-Token", "test-token-12345")
	w := httptest.NewRecorder()

	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var agents []*AgentConfig
	if err := json.Unmarshal(w.Body.Bytes(), &agents); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(agents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(agents))
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// TestUpdateAgentConfig проверяет обновление конфигурации агента
func TestUpdateAgentConfig(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	// Обновляем режим на SLEEP
	reqBody := map[string]interface{}{
		"mode":           "SLEEP",
		"sleep_interval": 120,
		"jitter":         15,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/agents/test-agent/config", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateAgentConfig(w, req, "test-agent")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Проверяем что изменения применились
	var updatedAgent AgentConfig
	if err := json.Unmarshal(w.Body.Bytes(), &updatedAgent); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if updatedAgent.Mode != StateSleep {
		t.Errorf("Expected Mode=SLEEP, got %s", updatedAgent.Mode)
	}

	if updatedAgent.SleepInterval != 120 {
		t.Errorf("Expected SleepInterval=120, got %d", updatedAgent.SleepInterval)
	}

	if updatedAgent.Jitter != 15 {
		t.Errorf("Expected Jitter=15, got %d", updatedAgent.Jitter)
	}
}

// TestUpdateAgentConfig_NotFound проверяет ошибку для несуществующего агента
func TestUpdateAgentConfig_NotFound(t *testing.T) {
	srv, _ := setupTestAPI(t)

	reqBody := map[string]interface{}{
		"mode": "SLEEP",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/agents/non-existent/config", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateAgentConfig(w, req, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestUpdateAgentConfig_InvalidMode проверяет валидацию режима
func TestUpdateAgentConfig_InvalidMode(t *testing.T) {
	srv, am := setupTestAPI(t)
	
	// Cleanup: ждём завершения всех async операций
	t.Cleanup(func() {
		time.Sleep(100 * time.Millisecond)
	})
	
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	reqBody := map[string]interface{}{
		"mode": "INVALID_MODE",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/agents/test-agent/config", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateAgentConfig(w, req, "test-agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestUpdateAgentConfig_InvalidInterval проверяет валидацию интервала сна
func TestUpdateAgentConfig_InvalidInterval(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	testCases := []int{0, -10, 86401}

	for _, interval := range testCases {
		reqBody := map[string]interface{}{
			"sleep_interval": interval,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/agents/test-agent/config", bytes.NewReader(body))
		req.Header.Set("X-Admin-Token", "test-token-12345")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.handleUpdateAgentConfig(w, req, "test-agent")

		if w.Code != http.StatusBadRequest {
			t.Errorf("For interval=%d: Expected status 400, got %d", interval, w.Code)
		}
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// TestUpdateAgentAlias проверяет обновление алиаса
func TestUpdateAgentAlias(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	reqBody := map[string]interface{}{
		"alias": "Production Server",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/agents/test-agent/config", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateAgentConfig(w, req, "test-agent")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var updatedAgent AgentConfig
	json.Unmarshal(w.Body.Bytes(), &updatedAgent)

	if updatedAgent.Alias != "Production Server" {
		t.Errorf("Expected Alias='Production Server', got '%s'", updatedAgent.Alias)
	}
}

// TestAPIDeleteAgent проверяет удаление агента через API
func TestAPIDeleteAgent(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	req := httptest.NewRequest("DELETE", "/api/agents/test-agent", nil)
	req.Header.Set("X-Admin-Token", "test-token-12345")
	w := httptest.NewRecorder()

	srv.handleDeleteAgent(w, req, "test-agent")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Проверяем что агент удалён
	if am.GetConfig("test-agent") != nil {
		t.Error("Agent should be deleted")
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// TestAPIDeleteAgent_NotFound проверяет удаление несуществующего агента через API
func TestAPIDeleteAgent_NotFound(t *testing.T) {
	srv, _ := setupTestAPI(t)

	req := httptest.NewRequest("DELETE", "/api/agents/non-existent", nil)
	req.Header.Set("X-Admin-Token", "test-token-12345")
	w := httptest.NewRecorder()

	srv.handleDeleteAgent(w, req, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// TestHealthCheck проверяет health endpoint
func TestHealthCheck(t *testing.T) {
	srv, _ := setupTestAPI(t)

	req := httptest.NewRequest("GET", "/health", nil)
	// Health endpoint не требует токен
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var health map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if health["status"] != "ok" {
		t.Errorf("Expected status='ok', got '%s'", health["status"])
	}
}

// TestUpdateAgentConfig_InvalidJSON проверяет обработку невалидного JSON
func TestUpdateAgentConfig_InvalidJSON(t *testing.T) {
	srv, am := setupTestAPI(t)
	am.RegisterAgent("test-agent", "192.168.1.1", "v3")

	req := httptest.NewRequest("POST", "/api/agents/test-agent/config", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-Admin-Token", "test-token-12345")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateAgentConfig(w, req, "test-agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	
	// Ждём async Save()
	time.Sleep(50 * time.Millisecond)
}

// ========================================
// Session API Tests (/api/sessions/{id})
// ========================================

func TestHandleSessions_SessionManagerNotAvailable(t *testing.T) {
	srv, am := setupTestAPI(t)
	_ = am
	srv.sessions = nil

	req := httptest.NewRequest("DELETE", "/api/sessions/test-agent", nil)
	w := httptest.NewRecorder()

	srv.handleSessions(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHandleSessions_DeleteSession_OK_ThenNotFound(t *testing.T) {
	srv, am, sm := setupTestAPIWithSessions(t)

	// Регистрируем агента в БД, чтобы не было "висячего" ID.
	am.RegisterAgent("test-agent", "127.0.0.1", "v3")

	// Создаём активную сессию + listener.
	_, sess, cleanup := newYamuxPair(t)
	defer cleanup()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	gen, _ := sm.RegisterSession("test-agent", sess, 50001, cancel)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	if ok := sm.SetListener("test-agent", gen, ln); !ok {
		t.Fatalf("SetListener should succeed for current session")
	}

	// 1) Убиваем активную сессию
	req := httptest.NewRequest("DELETE", "/api/sessions/test-agent", nil)
	w := httptest.NewRecorder()
	srv.handleSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 2) Повторное убийство — 404
	req2 := httptest.NewRequest("DELETE", "/api/sessions/test-agent", nil)
	w2 := httptest.NewRecorder()
	srv.handleSessions(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestHandleSessions_MissingSessionID(t *testing.T) {
	srv, _, _ := setupTestAPIWithSessions(t)

	req := httptest.NewRequest("DELETE", "/api/sessions/", nil)
	w := httptest.NewRecorder()

	srv.handleSessions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAgents_IncludesSessionInfo(t *testing.T) {
	srv, am, sm := setupTestAPIWithSessions(t)

	am.RegisterAgent("test-agent", "127.0.0.1", "v3")

	// Создаём активную сессию + listener, чтобы /api/agents отметил IsOnline и SocksAddr.
	_, sess, cleanup := newYamuxPair(t)
	defer cleanup()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	gen, _ := sm.RegisterSession("test-agent", sess, 50001, cancel)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	if ok := sm.SetListener("test-agent", gen, ln); !ok {
		t.Fatalf("SetListener should succeed for current session")
	}

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()
	srv.handleAgents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var agents []*AgentInfoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &agents); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("Expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "test-agent" {
		t.Fatalf("Expected agent id 'test-agent', got %q", agents[0].ID)
	}
	if !agents[0].IsOnline {
		t.Fatalf("Expected agent to be online when session+listener exist")
	}
	if agents[0].SocksAddr == "" {
		t.Fatalf("Expected SocksAddr to be set for online agent")
	}
}
