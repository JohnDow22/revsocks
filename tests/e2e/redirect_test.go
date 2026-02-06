package e2e

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TestE2E_ServerRedirectsUnknownProtocol проверяет stealth-поведение сервера:
// если входящее TCP-соединение не начинает handshake v3 ("AUTH"), сервер отвечает 301 редиректом.
func TestE2E_ServerRedirectsUnknownProtocol(t *testing.T) {
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	server := NewProcess(GlobalCtx.ServerPath, "server-redirect")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "redirectTest",
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
	}

	// Имитируем "сканер": шлём HTTP-заголовки вместо v3 handshake.
	conn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	if err != nil {
		t.Fatalf("Failed to write to server: %v", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	resp := string(buf[:n])

	if !strings.Contains(resp, "301") {
		t.Fatalf("Expected 301 redirect, got:\n%s", resp)
	}
	if !strings.Contains(resp, "Location: https://www.microsoft.com/") {
		t.Fatalf("Expected microsoft.com redirect, got:\n%s", resp)
	}
}

