package e2e

import (
	"fmt"
	"testing"
	"time"
)

// TestE2E_SocksAuth проверяет SOCKS5-аутентификацию на стороне агента.
// Контракт: при включённом -socks-auth подключение без кредов должно ломаться,
// а с корректными кредами — работать.
func TestE2E_SocksAuth(t *testing.T) {
	target, err := NewEchoServer()
	if err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}
	defer target.Close()

	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	password := "socksAuthTestPass"
	user := "user1"
	pass := "pass1"

	// Сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err = server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", password,
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
	}

	// Агент с включённой SOCKS auth
	agent := NewProcess(GlobalCtx.AgentPath, "agent")
	err = agent.Start(
		"-connect", serverAddr,
		"-pass", password,
		"-socks-auth",
		"-socks-user", user,
		"-socks-pass", pass,
	)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}
	defer agent.Stop()

	if err := agent.WaitForLog("Tunnel mode: accepting streams", 10*time.Second); err != nil {
		t.Fatalf("Agent didn't connect: %v\nAgent:\n%s\nServer:\n%s", err, agent.GetOutput(), server.GetOutput())
	}

	time.Sleep(400 * time.Millisecond)

	// 1) Без auth должно падать
	if err := TestProxyConnection(socksAddr, target.Addr, []byte("no-auth")); err == nil {
		t.Fatalf("Ожидали ошибку при подключении без SOCKS5 auth, но прокси-соединение прошло")
	}

	// 2) С корректным auth должно работать
	testData := []byte("with-auth-ok")
	if err := TestProxyConnectionWithAuth(socksAddr, target.Addr, testData, user, pass); err != nil {
		t.Fatalf("SOCKS auth proxy failed: %v\nAgent:\n%s\nServer:\n%s", err, agent.GetOutput(), server.GetOutput())
	}
}

