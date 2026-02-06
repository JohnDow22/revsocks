package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestE2E_CurlRealProxy проверяет работу SOCKS5 прокси через реальный curl
// Использует curl --socks5 для проверки HTTP/HTTPS запросов через туннель
// ПРИМЕЧАНИЕ: RevSocks сервер предоставляет SOCKS БЕЗ аутентификации
func TestE2E_CurlRealProxy(t *testing.T) {
	// Проверяем наличие curl
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found in PATH, skipping test")
	}

	// 1. Резервируем порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 2. Запускаем RevSocks сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	if err := server.WaitForLog("Starting to listen", 5*time.Second); err != nil {
		t.Fatalf("Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
	}
	t.Logf("✅ Server started on %s (SOCKS on %s)", serverAddr, socksAddr)

	// 3. Запускаем RevSocks клиент
	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	if err := client.WaitForLog("Tunnel mode: accepting streams", 5*time.Second); err != nil {
		t.Fatalf("Client didn't connect: %v\nClient logs:\n%s\nServer logs:\n%s",
			err, client.GetOutput(), server.GetOutput())
	}
	t.Logf("✅ Client connected")

	// Даём время на установку туннеля
	time.Sleep(500 * time.Millisecond)

	// 4. Тестируем через curl с SOCKS5
	// RevSocks SOCKS listener не требует аутентификации
	testURL := "https://google.com"
	
	cmd := exec.Command("curl",
		"-v",
		"--socks5", socksAddr,
		testURL,
		"--connect-timeout", "10",
		"--max-time", "20",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Logf("Curl output:\n%s", outputStr)
		t.Logf("Client logs:\n%s", client.GetOutput())
		t.Logf("Server logs:\n%s", server.GetOutput())
		t.Fatalf("Curl failed: %v", err)
	}

	// 5. Проверяем что в выводе есть признаки успешного подключения
	// google.com может возвращать 301/302/200
	if !strings.Contains(outputStr, "200") && 
	   !strings.Contains(outputStr, "301") && 
	   !strings.Contains(outputStr, "302") &&
	   !strings.Contains(outputStr, "HTTP/") {
		t.Logf("Curl output:\n%s", outputStr)
		t.Fatal("Curl response doesn't contain expected HTTP status")
	}

	// Проверяем что curl использовал SOCKS5
	if !strings.Contains(outputStr, "SOCKS") && 
	   !strings.Contains(outputStr, "Connected to") {
		t.Logf("Warning: Can't verify SOCKS5 usage in curl output")
	}

	t.Logf("✅ Curl successfully connected through SOCKS5 proxy")
	t.Logf("Curl output (first 500 chars):\n%s", truncateString(outputStr, 500))
}

// TestE2E_CurlRealProxyNoAuth проверяет HTTPS запрос через прокси
func TestE2E_CurlRealProxyHTTPS(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found in PATH, skipping test")
	}

	// 1. Порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 2. Сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	server.WaitForLog("Starting to listen", 5*time.Second)
	t.Log("✅ Server started")

	// 3. Клиент
	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	client.WaitForLog("Received ACK", 5*time.Second)
	t.Log("✅ Client connected")

	time.Sleep(500 * time.Millisecond)

	// 4. Curl HTTPS запрос
	cmd := exec.Command("curl",
		"-v",
		"--socks5", socksAddr,
		"https://www.google.com",
		"--connect-timeout", "10",
		"--max-time", "20",
		"-L", // Follow redirects
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	if err != nil {
		t.Logf("Curl output:\n%s", outputStr)
		t.Fatalf("Curl failed: %v", err)
	}

	// 5. Проверяем успех (google может вернуть редирект или 200)
	if !strings.Contains(outputStr, "HTTP/") {
		t.Fatalf("No HTTP response detected")
	}

	t.Log("✅ Curl HTTPS works through SOCKS5")
}

// TestE2E_CurlRealProxyHTTP проверяет HTTP запрос через прокси
func TestE2E_CurlRealProxyHTTP(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found in PATH")
	}

	// 1. Порты
	agentPort := GetFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", agentPort)
	socksPort := GetFreePort(t)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	// 2. Сервер
	server := NewProcess(GlobalCtx.ServerPath, "server")
	err := server.Start(
		"-listen", serverAddr,
		"-socks", socksAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	server.WaitForLog("Starting to listen", 5*time.Second)

	// 3. Клиент
	client := NewProcess(GlobalCtx.AgentPath, "agent")
	err = client.Start(
		"-connect", serverAddr,
		"-pass", "testpass123",
	)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Stop()

	client.WaitForLog("Received ACK", 5*time.Second)
	time.Sleep(500 * time.Millisecond)

	// 4. HTTP запрос (менее безопасно, но работает быстрее)
	cmd := exec.Command("curl",
		"-v",
		"--socks5", socksAddr,
		"http://example.com",
		"--connect-timeout", "10",
		"--max-time", "20",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Logf("Curl output:\n%s", outputStr)
		t.Fatalf("HTTP curl failed: %v", err)
	}

	// Проверяем успех
	if !strings.Contains(outputStr, "200") && 
	   !strings.Contains(outputStr, "HTTP/") {
		t.Fatalf("No successful HTTP response")
	}

	t.Log("✅ Curl HTTP works")
}

// truncateString обрезает строку до maxLen символов
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
