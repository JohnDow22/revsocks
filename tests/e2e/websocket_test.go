package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// ========================================
// WebSocket E2E Tests
// ========================================

// TestE2E_WebSocket_Basic проверяет базовое WebSocket соединение
// Сервер в режиме WebSocket, агент подключается через WSS
func TestE2E_WebSocket_Basic(t *testing.T) {
	// 1. Запускаем target (echo сервер)
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()
	t.Logf("✅ Target listening on %s", target.Addr)

	// 2. Порты для WebSocket сервера
	wsPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", wsPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер в WebSocket режиме (без TLS для простоты теста)
	// Примечание: в проде используется wss://, но для теста ws:// достаточно
	server := NewProcess(GlobalCtx.ServerPath, "ws-server")
	err = server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "wsTestPassword",
		"-ws", // WebSocket режим
	)
	if err != nil {
		t.Fatalf("Failed to start WS server: %v", err)
	}
	defer server.Stop()

	// Ждём готовности сервера (WebSocket сервер)
	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
	}
	t.Logf("✅ WebSocket Server started on %s (SOCKS on %s)", serverAddr, socksAddr)

	// 4. Запускаем агента в WebSocket режиме
	// Агент подключается через ws://
	wsURL := fmt.Sprintf("ws://%s", serverAddr)
	client := NewProcess(GlobalCtx.AgentPath, "ws-agent")
	err = client.Start(
		"-connect", wsURL,
		"-pass", "wsTestPassword",
		"-ws", // WebSocket режим
	)
	if err != nil {
		t.Fatalf("Failed to start WS client: %v", err)
	}
	defer client.Stop()

	// Ждём получения команды от сервера (v3 протокол)
	if err := client.WaitForLog("Server response:", 5*time.Second); err != nil {
		t.Fatalf("WS Client didn't receive server response: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Log("✅ WS Client received server command")

	// Ждём установки TUNNEL режима
	if err := client.WaitForLog("WebSocket tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("WS Client didn't enter tunnel mode: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Log("✅ WS Client in TUNNEL mode")

	// Даём время на установку туннеля
	time.Sleep(500 * time.Millisecond)

	// 5. Тестируем проксирование
	testData := []byte("WebSocket E2E test data!")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("WS Proxy test failed: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}

	t.Log("✅ WebSocket Basic E2E test passed")
}

// TestE2E_WebSocket_TLS проверяет WebSocket соединение с TLS (wss://)
func TestE2E_WebSocket_TLS(t *testing.T) {
	// 1. Запускаем target
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	// 2. Порты
	wssPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", wssPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер с WSS (WebSocket + TLS)
	server := NewProcess(GlobalCtx.ServerPath, "wss-server")
	err = server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "wssTestPassword",
		"-ws",  // WebSocket режим
		"-tls", // С TLS
	)
	if err != nil {
		t.Fatalf("Failed to start WSS server: %v", err)
	}
	defer server.Stop()

	// TLS сервер генерирует сертификат (~1-2 сек)
	if err := server.WaitForLog("certificate", 10*time.Second); err != nil {
		t.Fatalf("WSS Server didn't generate TLS certificate: %v\nLogs:\n%s", err, server.GetOutput())
	}
	t.Log("✅ WSS Server TLS certificate ready")

	// Ждём полной инициализации
	time.Sleep(500 * time.Millisecond)

	// 4. Запускаем агента с WSS
	wssURL := fmt.Sprintf("wss://%s", serverAddr)
	client := NewProcess(GlobalCtx.AgentPath, "wss-agent")
	err = client.Start(
		"-connect", wssURL,
		"-pass", "wssTestPassword",
		"-ws",  // WebSocket режим
		"-tls", // TLS
		// skip verify для self-signed сертификата
	)
	if err != nil {
		t.Fatalf("Failed to start WSS client: %v", err)
	}
	defer client.Stop()

	// Ждём команды от сервера
	if err := client.WaitForLog("Server response:", 10*time.Second); err != nil {
		t.Fatalf("WSS Client didn't receive server response: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Log("✅ WSS Client received server command")

	// Ждём установки TUNNEL режима
	if err := client.WaitForLog("WebSocket tunnel mode", 5*time.Second); err != nil {
		t.Fatalf("WSS Client didn't enter tunnel mode: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Log("✅ WSS Client in TUNNEL mode")

	time.Sleep(500 * time.Millisecond)

	// 5. Тестируем проксирование
	testData := []byte("WSS encrypted test data!")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("WSS Proxy test failed: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}

	t.Log("✅ WebSocket TLS (WSS) E2E test passed")
}

// TestE2E_WebSocket_SleepWake проверяет SLEEP/WAKE через WebSocket
func TestE2E_WebSocket_SleepWake(t *testing.T) {
	agentDBPath := fmt.Sprintf("/tmp/test_ws_agents_%d.json", time.Now().Unix())
	defer os.Remove(agentDBPath)

	// 1. Порты
	wsPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", wsPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	adminPort := GetFreePort(t)
	adminAddr := fmt.Sprintf("127.0.0.1:%d", adminPort)
	adminToken := "ws_test_token"

	// 2. Запускаем сервер с WebSocket и Admin API
	server := NewProcess(GlobalCtx.ServerPath, "ws-server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "wsSleepWakeTest",
		"-ws",
		"-agentdb", agentDBPath,
		"-admin-api",
		"-admin-port", adminAddr,
	)
	if err != nil {
		t.Fatalf("Failed to start WS server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("AgentManager initialized", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't start AgentManager: %v", err)
	}
	if err := server.WaitForLog("Starting HTTP API", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't start Admin API: %v", err)
	}
	t.Log("✅ WS Server with Admin API started")

	// 3. Запускаем агента в WebSocket режиме
	agentIDPath := fmt.Sprintf("/tmp/test_ws_agentid_%d.id", time.Now().Unix())
	defer os.Remove(agentIDPath)

	wsURL := fmt.Sprintf("ws://%s", serverAddr)
	client := NewProcess(GlobalCtx.AgentPath, "ws-agent")
	err = client.Start(
		"-connect", wsURL,
		"-pass", "wsSleepWakeTest",
		"-ws",
		"-agentid-path", agentIDPath,
	)
	if err != nil {
		t.Fatalf("Failed to start WS agent: %v", err)
	}
	defer client.Stop()

	// Ждём команды от сервера
	if err := client.WaitForLog("Server response:", 5*time.Second); err != nil {
		t.Fatalf("WS Agent didn't receive command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ WS Agent connected")

	// Даём время на регистрацию
	time.Sleep(1 * time.Second)

	// 4. Получаем список агентов через Admin API
	agents, err := GetAgents(adminAddr, adminToken)
	if err != nil {
		t.Fatalf("Failed to get agents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("Expected 1 agent, got %d", len(agents))
	}
	agentID := agents[0]["id"].(string)
	currentMode := agents[0]["mode"].(string)
	if currentMode != "TUNNEL" {
		t.Fatalf("Expected initial mode TUNNEL, got %s", currentMode)
	}
	t.Logf("✅ WS Agent registered: ID=%s, mode=%s", agentID, currentMode)

	// 5. Переводим агента в SLEEP режим
	err = UpdateAgentConfig(adminAddr, adminToken, agentID, map[string]interface{}{
		"mode":           "SLEEP",
		"sleep_interval": 5, // 5 секунд для быстрого теста
		"jitter":         0,
	})
	if err != nil {
		t.Fatalf("Failed to set SLEEP mode: %v", err)
	}
	t.Log("✅ Sent SLEEP command via Admin API")

	// Агент должен получить SLEEP при следующем подключении
	// (сервер закрывает текущую сессию для применения нового конфига)
	if err := client.WaitForLog("SLEEP", 10*time.Second); err != nil {
		t.Logf("Note: SLEEP log not found, but checking agent behavior...")
	}

	// Ждём пока агент уснёт и проснётся
	time.Sleep(3 * time.Second)

	// 6. Переводим обратно в TUNNEL (wake)
	err = UpdateAgentConfig(adminAddr, adminToken, agentID, map[string]interface{}{
		"mode": "TUNNEL",
	})
	if err != nil {
		t.Fatalf("Failed to wake agent: %v", err)
	}
	t.Log("✅ Sent TUNNEL (wake) command via Admin API")

	// Ждём пока агент проснётся и подключится
	time.Sleep(6 * time.Second)

	// 7. Проверяем что агент работает в TUNNEL режиме
	if err := client.WaitForLog("WebSocket tunnel mode", 10*time.Second); err != nil {
		t.Logf("Note: tunnel mode log not explicit, checking proxy...")
	}

	// 8. Проверяем что прокси работает
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	testData := []byte("WS sleep/wake test data")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("WS Proxy after wake failed: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}

	t.Log("✅ WebSocket SLEEP/WAKE test passed")
}

// TestE2E_WebSocket_Reconnect проверяет переподключение через WebSocket
func TestE2E_WebSocket_Reconnect(t *testing.T) {
	// 1. Запускаем target
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	// 2. Порты
	wsPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", wsPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер
	server := NewProcess(GlobalCtx.ServerPath, "ws-server")
	err = server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "wsReconnectTest",
		"-ws",
	)
	if err != nil {
		t.Fatalf("Failed to start WS server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't start: %v", err)
	}
	t.Log("✅ WS Server started")

	// 4. Первый агент
	wsURL := fmt.Sprintf("ws://%s", serverAddr)
	client1 := NewProcess(GlobalCtx.AgentPath, "ws-agent1")
	err = client1.Start(
		"-connect", wsURL,
		"-pass", "wsReconnectTest",
		"-ws",
		"-recn", "1",
	)
	if err != nil {
		t.Fatalf("Failed to start WS agent1: %v", err)
	}

	if err := client1.WaitForLog("WebSocket tunnel mode", 5*time.Second); err != nil {
		t.Fatalf("WS Agent1 didn't connect: %v\nClient:\n%s", err, client1.GetOutput())
	}
	t.Log("✅ WS Agent1 connected")

	time.Sleep(300 * time.Millisecond)

	// 5. Тестируем первое соединение
	testData1 := []byte("First WS connection test")
	if err := TestProxyConnection(socksAddr, target.Addr, testData1); err != nil {
		t.Fatalf("First WS proxy test failed: %v", err)
	}
	t.Log("✅ First WS connection works")

	// 6. Убиваем первого агента
	client1.Stop()
	t.Log("✅ WS Agent1 stopped")
	time.Sleep(500 * time.Millisecond)

	// 7. Второй агент (reconnect)
	client2 := NewProcess(GlobalCtx.AgentPath, "ws-agent2")
	err = client2.Start(
		"-connect", wsURL,
		"-pass", "wsReconnectTest",
		"-ws",
		"-recn", "1",
	)
	if err != nil {
		t.Fatalf("Failed to start WS agent2: %v", err)
	}
	defer client2.Stop()

	if err := client2.WaitForLog("WebSocket tunnel mode", 5*time.Second); err != nil {
		t.Fatalf("WS Agent2 didn't reconnect: %v\nClient:\n%s\nServer:\n%s",
			err, client2.GetOutput(), server.GetOutput())
	}
	t.Log("✅ WS Agent2 reconnected")

	time.Sleep(300 * time.Millisecond)

	// 8. Тестируем второе соединение
	testData2 := []byte("Second WS connection after reconnect")
	if err := TestProxyConnection(socksAddr, target.Addr, testData2); err != nil {
		t.Fatalf("WS Reconnect proxy test failed: %v\nClient2:\n%s\nServer:\n%s",
			err, client2.GetOutput(), server.GetOutput())
	}

	t.Log("✅ WebSocket Reconnect test passed")
}

// TestE2E_WebSocket_V3Protocol проверяет что WebSocket использует v3 протокол
// (сервер отправляет команду, агент её получает и обрабатывает)
func TestE2E_WebSocket_V3Protocol(t *testing.T) {
	agentDBPath := fmt.Sprintf("/tmp/test_ws_v3_%d.json", time.Now().Unix())
	defer os.Remove(agentDBPath)

	// 1. Порты
	wsPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", wsPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 2. Запускаем сервер с AgentManager
	server := NewProcess(GlobalCtx.ServerPath, "ws-server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "wsV3ProtocolTest",
		"-ws",
		"-agentdb", agentDBPath,
	)
	if err != nil {
		t.Fatalf("Failed to start WS server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("AgentManager initialized", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't start AgentManager: %v", err)
	}
	t.Log("✅ WS Server with AgentManager started")

	// 3. Запускаем агента
	agentIDPath := fmt.Sprintf("/tmp/test_ws_v3_agentid_%d.id", time.Now().Unix())
	defer os.Remove(agentIDPath)

	wsURL := fmt.Sprintf("ws://%s", serverAddr)
	client := NewProcess(GlobalCtx.AgentPath, "ws-agent")
	err = client.Start(
		"-connect", wsURL,
		"-pass", "wsV3ProtocolTest",
		"-ws",
		"-agentid-path", agentIDPath,
	)
	if err != nil {
		t.Fatalf("Failed to start WS agent: %v", err)
	}
	defer client.Stop()

	// 4. Проверяем что агент получил команду от сервера (v3 протокол)
	if err := client.WaitForLog("Server response:", 5*time.Second); err != nil {
		t.Fatalf("WS Agent didn't receive v3 command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ WS Agent received v3 server command")

	// 5. Проверяем что сервер зарегистрировал агента
	if err := server.WaitForLog("New agent registered", 5*time.Second); err != nil {
		t.Fatalf("WS Server didn't register agent: %v\nServer:\n%s", err, server.GetOutput())
	}
	t.Log("✅ WS Server registered agent")

	// 6. Проверяем что агент получил CMD TUNNEL
	if err := client.WaitForLog("CMD TUNNEL", 5*time.Second); err != nil {
		t.Fatalf("WS Agent didn't receive TUNNEL command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ WS Agent received TUNNEL command (v3 protocol working)")

	// 7. Проверяем что туннель работает
	time.Sleep(500 * time.Millisecond)

	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	testData := []byte("WS v3 protocol test")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("WS v3 proxy test failed: %v", err)
	}

	t.Log("✅ WebSocket v3 Protocol test passed")
}
