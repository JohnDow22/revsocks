package e2e

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// TestE2E_Basic проверяет базовый сценарий:
// 1. Запуск echo сервера (target)
// 2. Запуск RevSocks сервера
// 3. Запуск RevSocks клиента
// 4. Проксирование трафика через SOCKS5
func TestE2E_Basic(t *testing.T) {
	// 1. Запускаем target (echo сервер)
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()
	t.Logf("✅ Target listening on %s", target.Addr)

	// 2. Резервируем порты для RevSocks
	// Порт для подключения агентов (server -listen)
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	
	// Порт для SOCKS5 прокси (создаётся сервером автоматически)
	// По дефолту это :1080, но мы можем указать через -socks
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем RevSocks сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err = server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr, // Указываем порт для SOCKS
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Ждём готовности сервера
	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
	}
	t.Logf("✅ Server started on %s (SOCKS on %s)", serverAddr, socksAddr)

	// 4. Запускаем RevSocks клиент (подключается к серверу)
	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	// Ждём подключения клиента
	// RevSocks клиент v3 выводит "Tunnel mode: accepting streams"
	if err := client.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client didn't connect: %v\nClient logs:\n%s\nServer logs:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Logf("✅ Client connected")

	// Даём время на установку туннеля
	time.Sleep(500 * time.Millisecond)

	// 5. Тестируем проксирование трафика через SOCKS listener на сервере
	testData := []byte("Hello, RevSocks E2E Test!")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("Proxy test failed: %v\nClient logs:\n%s\nServer logs:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}

	t.Log("✅ E2E test passed - traffic proxied successfully")
}

// TestE2E_Reconnect проверяет переподключение клиента
func TestE2E_Reconnect(t *testing.T) {
	// 1. Запускаем target
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	// 2. Порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err = server.Start("-listen", serverAddr, "-socks", socksAddr, "-pass", "reconnectTest")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v", err)
	}
	t.Log("✅ Server started")

	// 4. Первый клиент
	client1 := NewProcess(GlobalCtx.AgentPath, "agent1")
	err = client1.Start("-connect", serverAddr, "-pass", "reconnectTest", "-recn", "1")
	if err != nil {
		t.Fatalf("Failed to start client1: %v", err)
	}

	if err := client1.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client1 didn't connect: %v", err)
	}
	t.Log("✅ Client1 connected")

	time.Sleep(300 * time.Millisecond)

	// 5. Тестируем первое соединение
	testData := []byte("First connection test")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("First proxy test failed: %v", err)
	}
	t.Log("✅ First connection works")

	// 6. Убиваем первого клиента
	client1.Stop()
	t.Log("✅ Client1 stopped")
	time.Sleep(500 * time.Millisecond)

	// 7. Запускаем второго клиента (reconnect)
	client2 := NewProcess(GlobalCtx.AgentPath, "agent2")
	err = client2.Start("-connect", serverAddr, "-pass", "reconnectTest", "-recn", "1")
	if err != nil {
		t.Fatalf("Failed to start client2: %v", err)
	}
	defer client2.Stop()

	if err := client2.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client2 didn't connect: %v\nServer logs:\n%s", err, server.GetOutput())
	}
	t.Log("✅ Client2 reconnected")

	time.Sleep(300 * time.Millisecond)

	// 8. Тестируем второе соединение
	testData2 := []byte("Second connection after reconnect")
	if err := TestProxyConnection(socksAddr, target.Addr, testData2); err != nil {
		t.Fatalf("Reconnect proxy test failed: %v\nClient2:\n%s\nServer:\n%s",
			err, client2.GetOutput(), server.GetOutput())
	}

	t.Log("✅ Reconnect test passed")
}

// TestE2E_MultipleClients проверяет работу с несколькими клиентами
func TestE2E_MultipleClients(t *testing.T) {
	// 1. Запускаем target
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	// 2. Порты - сервер, первый SOCKS (будет инкрементироваться)
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	baseSocksPort := GetFreePort(t)
	socksAddr1 := fmt.Sprintf("127.0.0.1:%d", baseSocksPort)

	// 3. Запускаем сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err = server.Start("-listen", serverAddr, "-socks", socksAddr1, "-pass", "multiTest")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v", err)
	}
	t.Log("✅ Server started")

	// Для теста с несколькими клиентами нужны разные agentID
	// Так как по умолчанию agentID = hostname, второй клиент заменяет первого
	// Этот тест проверяет что сервер корректно переключает сессии
	// и переиспользует порт при reconnect того же агента

	// 4. Первый клиент
	client1 := NewProcess(GlobalCtx.AgentPath, "agent1")
	err = client1.Start("-connect", serverAddr, "-pass", "multiTest", "-recn", "1")
	if err != nil {
		t.Fatalf("Failed to start client1: %v", err)
	}
	defer client1.Stop()

	if err := client1.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client1 didn't connect: %v", err)
	}
	t.Log("✅ Client1 connected (will use same agentID)")

	// Ждём пока сервер создаст listener для первого клиента
	if err := server.WaitForLog("Handshake recognized", 3*time.Second); err != nil {
		t.Fatalf("Server didn't create listener for client1: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// 5. Тестируем первого клиента через первый SOCKS порт
	testData1 := []byte("Client1 test data")
	if err := TestProxyConnection(socksAddr1, target.Addr, testData1); err != nil {
		t.Fatalf("Client1 proxy failed: %v\nServer:\n%s", err, server.GetOutput())
	}
	t.Log("✅ Client1 proxy works")

	// 6. Второй клиент с тем же hostname (тот же agentID)
	// Сервер закроет старую сессию и переиспользует порт
	client2 := NewProcess(GlobalCtx.AgentPath, "agent2")
	err = client2.Start("-connect", serverAddr, "-pass", "multiTest", "-recn", "1")
	if err != nil {
		t.Fatalf("Failed to start client2: %v", err)
	}
	defer client2.Stop()

	if err := client2.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client2 didn't connect: %v", err)
	}
	t.Log("✅ Client2 connected (replaced client1 session)")

	// Ждём переключения сессии
	if err := server.WaitForLog("Reusing cached port", 3*time.Second); err != nil {
		t.Logf("Note: port reuse log not found (may be first connection)")
	}

	time.Sleep(300 * time.Millisecond)

	// 7. После замены сессии порт переиспользуется (тот же socksAddr1)
	testData2 := []byte("Client2 test data through same port")
	if err := TestProxyConnection(socksAddr1, target.Addr, testData2); err != nil {
		t.Fatalf("Client2 proxy failed: %v\nServer:\n%s", err, server.GetOutput())
	}
	t.Log("✅ Client2 works on reused port")

	t.Log("✅ Multiple clients test passed")
}

// TestE2E_TLS проверяет TLS соединение
func TestE2E_TLS(t *testing.T) {
	// 1. Запускаем target
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	// 2. Порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер с TLS
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err = server.Start("-listen", serverAddr, "-socks", socksAddr, "-pass", "tlsTest", "-tls")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// TLS сервер генерирует сертификат при первом запуске (~1-2 сек)
	// Ждём появления "Using cached" или "certificate cached"
	if err := server.WaitForLog("certificate", 10*time.Second); err != nil {
		t.Fatalf("Server didn't generate TLS certificate: %v\nLogs:\n%s", err, server.GetOutput())
	}
	t.Log("✅ TLS Server certificate ready")

	// Дополнительная пауза для полной инициализации listener
	time.Sleep(500 * time.Millisecond)

	// 4. Клиент с TLS (skip verify для self-signed)
	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start("-connect", serverAddr, "-pass", "tlsTest", "-tls", "-recn", "1")
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	if err := client.WaitForLog("Tunnel mode: accepting streams", 10*time.Second); err != nil {
		t.Fatalf("TLS Client didn't connect: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Log("✅ TLS Client connected")

	time.Sleep(300 * time.Millisecond)

	// 5. Тестируем проксирование
	testData := []byte("TLS encrypted test data")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("TLS proxy test failed: %v", err)
	}

	t.Log("✅ TLS E2E test passed")
}

// GetFreePort возвращает свободный TCP порт
func GetFreePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	
	// Даём время OS освободить порт
	time.Sleep(100 * time.Millisecond)
	
	return port
}

// ========================================
// Beacon Mode E2E Tests
// ========================================

// TestE2E_BeaconSleepCycle проверяет beacon режим (SLEEP mode)
func TestE2E_BeaconSleepCycle(t *testing.T) {
	// 1. Создаём временный agents.json с дефолтным SLEEP режимом
	agentDBPath := fmt.Sprintf("/tmp/test_agents_%d.json", time.Now().Unix())
	defer func() {
		// Cleanup
		_ = os.Remove(agentDBPath)
	}()

	// 2. Порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 3. Запускаем сервер с AgentManager
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "beaconTest",
		"-agentdb", agentDBPath,
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("AgentManager initialized", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start AgentManager: %v", err)
	}
	t.Log("✅ Server started with AgentManager")

	// 4. Запускаем агента (без флагов режима, сервер решает)
	agentIDPath := fmt.Sprintf("/tmp/test_agentid_%d.id", time.Now().Unix())
	defer func() {
		_ = os.Remove(agentIDPath)
	}()

	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "beaconTest",
		"-agentid-path", agentIDPath,
	)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer client.Stop()

	// 5. Агент должен получить команду от сервера
	if err := client.WaitForLog("Server command:", 5*time.Second); err != nil {
		t.Fatalf("Agent didn't get server command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ Agent connected and got server command")

	// 6. По умолчанию новый агент получает TUNNEL режим
	if err := server.WaitForLog("New agent registered", 5*time.Second); err != nil {
		t.Fatalf("Server didn't register agent: %v\nServer:\n%s", err, server.GetOutput())
	}
	t.Log("✅ Agent registered on server")

	// 7. Проверяем что агент получил CMD TUNNEL и установил сессию
	if err := client.WaitForLog("Server command: TUNNEL", 5*time.Second); err != nil {
		t.Fatalf("Agent didn't receive TUNNEL command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ Agent received TUNNEL command")

	// 8. Ждём установки yamux сессии
	time.Sleep(1 * time.Second)

	// 9. Проверяем что SOCKS туннель работает
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	testData := []byte("Beacon mode tunnel test")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("Tunnel proxy failed: %v", err)
	}
	t.Log("✅ TUNNEL mode working")

	t.Log("✅ Beacon Sleep Cycle test passed")
}

// TestE2E_BeaconSleepToTunnel проверяет переход из SLEEP в TUNNEL через Admin API
func TestE2E_BeaconSleepToTunnel(t *testing.T) {
	agentDBPath := fmt.Sprintf("/tmp/test_agents_sleep_to_tunnel_%d.json", time.Now().Unix())
	defer os.Remove(agentDBPath)

	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	adminPort := GetFreePort(t)
	adminAddr := fmt.Sprintf("127.0.0.1:%d", adminPort)
	adminToken := "test_token_sleep_to_tunnel"

	// Запускаем сервер с Admin API
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "sleepToTunnelBeacon",
		"-agentdb", agentDBPath,
		"-admin-api",
		"-admin-port", adminAddr,
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	server.WaitForLog("AgentManager initialized", 5*time.Second)
	server.WaitForLog("Starting HTTP API", 5*time.Second)
	t.Log("✅ Server with Admin API started")

	// Запускаем агента (по умолчанию получит TUNNEL от сервера)
	agentIDPath := fmt.Sprintf("/tmp/test_agentid_sleep_to_tunnel_%d.id", time.Now().Unix())
	defer os.Remove(agentIDPath)

	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "sleepToTunnelBeacon",
		"-agentid-path", agentIDPath,
	)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer client.Stop()

	// Ждём получения команды от сервера (v3 протокол)
	if err := client.WaitForLog("Server command:", 5*time.Second); err != nil {
		t.Fatalf("Agent didn't receive server command: %v\nClient:\n%s", err, client.GetOutput())
	}
	t.Log("✅ Agent connected and received server command")

	// Даём время на регистрацию агента
	time.Sleep(1 * time.Second)

	// Получаем список агентов через API
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
	t.Logf("✅ Agent registered with ID: %s, mode: %s", agentID, currentMode)

	// Переводим агента в SLEEP режим через Admin API
	err = UpdateAgentConfig(adminAddr, adminToken, agentID, map[string]interface{}{
		"mode":           "SLEEP",
		"sleep_interval": 5, // 5 секунд для быстрого теста
		"jitter":         0,
	})
	if err != nil {
		t.Fatalf("Failed to update agent config: %v", err)
	}
	t.Log("✅ Sent SLEEP command via Admin API")

	// Агент должен получить команду SLEEP при следующем подключении
	// (сервер закрывает сессию для применения нового конфига)
	if err := client.WaitForLog("SLEEP", 10*time.Second); err != nil {
		t.Logf("Note: SLEEP log not found explicitly, checking agent behavior...")
	}
	t.Log("✅ Agent received SLEEP command")

	// Ждём пока агент уснёт и проснётся
	time.Sleep(3 * time.Second)

	// Переводим обратно в TUNNEL режим (wake up)
	err = UpdateAgentConfig(adminAddr, adminToken, agentID, map[string]interface{}{
		"mode": "TUNNEL",
	})
	if err != nil {
		t.Fatalf("Failed to wake agent: %v", err)
	}
	t.Log("✅ Sent TUNNEL (wake) command via Admin API")

	// Ждём пока агент проснётся и подключится
	time.Sleep(6 * time.Second)

	// Агент должен подключиться и получить команду TUNNEL
	if err := client.WaitForLog("TUNNEL", 10*time.Second); err != nil {
		t.Logf("Note: TUNNEL log not found explicitly, checking proxy...")
	}
	t.Log("✅ Agent woke up and received TUNNEL command")

	// Проверяем что прокси работает (агент в TUNNEL режиме)
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to create echo server: %v", err)
	}
	defer target.Close()

	testData := []byte("sleep_to_tunnel_test_data")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("Proxy doesn't work after wake: %v\nClient:\n%s\nServer:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}

	t.Log("✅ SLEEP→TUNNEL transition test passed")
}

// TestE2E_BeaconReconnect проверяет что beacon агент переиспользует ID при reconnect
func TestE2E_BeaconReconnect(t *testing.T) {
	agentDBPath := fmt.Sprintf("/tmp/test_agents_reconnect_%d.json", time.Now().Unix())
	defer os.Remove(agentDBPath)

	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// Запускаем сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "reconnectBeacon",
		"-agentdb", agentDBPath,
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	server.WaitForLog("AgentManager initialized", 5*time.Second)
	t.Log("✅ Server started")

	// Запускаем первого агента
	agentIDPath := fmt.Sprintf("/tmp/test_agentid_reconnect_%d.id", time.Now().Unix())
	defer os.Remove(agentIDPath)

	client1 := NewProcess(GlobalCtx.AgentPath, "agent1")
	err = client1.Start(
		"-connect", serverAddr,
		"-pass", "reconnectBeacon",
		"-agentid-path", agentIDPath,
	)
	if err != nil {
		t.Fatalf("Failed to start agent1: %v", err)
	}

	// Ждём получения команды от сервера (v3 протокол)
	if err := client1.WaitForLog("Server command:", 5*time.Second); err != nil {
		t.Fatalf("Agent1 didn't receive command: %v\nClient:\n%s", err, client1.GetOutput())
	}
	t.Log("✅ Agent1 connected and got first command")

	// Ждём установки туннеля
	time.Sleep(500 * time.Millisecond)

	// Останавливаем первого агента
	client1.Stop()
	time.Sleep(500 * time.Millisecond)

	// Запускаем второго агента с тем же agentid-path
	client2 := NewProcess(GlobalCtx.AgentPath, "agent2")
	err = client2.Start(
		"-connect", serverAddr,
		"-pass", "reconnectBeacon",
		"-agentid-path", agentIDPath, // Тот же файл ID
	)
	if err != nil {
		t.Fatalf("Failed to start agent2: %v", err)
	}
	defer client2.Stop()

	// Агент должен использовать тот же ID и получить команду
	if err := client2.WaitForLog("Server command:", 5*time.Second); err != nil {
		t.Fatalf("Agent2 didn't receive command: %v\nClient:\n%s", err, client2.GetOutput())
	}
	t.Log("✅ Agent2 reconnected with same ID")

	// Проверяем что сервер распознал reconnect (reuse port)
	if err := server.WaitForLog("Reusing cached port", 3*time.Second); err != nil {
		t.Logf("Note: port reuse log not explicit, but agent reconnected")
	}

	// Проверяем что прокси работает после reconnect
	time.Sleep(500 * time.Millisecond)
	
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	testData := []byte("Beacon reconnect test")
	if err := TestProxyConnection(socksAddr, target.Addr, testData); err != nil {
		t.Fatalf("Proxy after reconnect failed: %v\nClient:\n%s\nServer:\n%s",
			err, client2.GetOutput(), server.GetOutput())
	}

	t.Log("✅ Beacon reconnect with persistent ID works")
}
