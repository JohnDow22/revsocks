# 04_Scenarios.md

## A. Context Setup
*   **Target File:** `tests/e2e/scenarios_test.go`
*   **Target File:** `tests/e2e/traffic.go`
*   **Target File:** `tests/e2e/target.go`

## B. Detailed Design
Здесь мы соединяем всё вместе.
`Target` — это сервер назначения.
`Traffic` — это клиент, который проверяет канал.
`Scenarios` — это тесты.

### 1. `target.go`
Echo-сервер на TCP, возвращающий полученные данные.
```go
type EchoServer struct {
    listener net.Listener
    Addr     string // Реальный адрес после Listen(":0")
}

func NewEchoServer() (*EchoServer, error) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return nil, err
    }
    
    es := &EchoServer{
        listener: ln,
        Addr:     ln.Addr().String(),
    }
    
    go es.serve()
    return es, nil
}

func (es *EchoServer) serve() {
    for {
        conn, err := es.listener.Accept()
        if err != nil {
            return // Listener closed
        }
        go func(c net.Conn) {
            defer c.Close()
            io.Copy(c, c) // Echo back
        }(conn)
    }
}
```

### 2. `traffic.go`
SOCKS5 клиент для проверки проксирования.
```go
import "golang.org/x/net/proxy"

// TestProxyConnection подключается к targetAddr через SOCKS5 proxy
func TestProxyConnection(proxyAddr, targetAddr string, testData []byte) error {
    dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
    if err != nil {
        return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
    }
    
    conn, err := dialer.Dial("tcp", targetAddr)
    if err != nil {
        return fmt.Errorf("failed to dial through proxy: %w", err)
    }
    defer conn.Close()
    
    // Send data
    if _, err := conn.Write(testData); err != nil {
        return fmt.Errorf("write failed: %w", err)
    }
    
    // Read echo response
    buf := make([]byte, len(testData))
    if _, err := io.ReadFull(conn, buf); err != nil {
        return fmt.Errorf("read failed: %w", err)
    }
    
    if !bytes.Equal(buf, testData) {
        return fmt.Errorf("data mismatch: got %v, want %v", buf, testData)
    }
    
    return nil
}
```

### 3. `scenarios_test.go`
Тесты через `t.Run`.
*   **TestConnect:** Базовый сценарий. Server bind :0 -> Client connect -> Proxy connect -> Echo OK.
*   **TestReconnect:** Server stop -> Client wait -> Server start -> Proxy OK.
*   **TestAuth:** (Если реализовано) Проверка пароля.

## C. Implementation Steps

### 1. Create `target.go`
Эхо-сервер на случайном порту.

### 2. Create `traffic.go`
SOCKS5 клиент.

### 3. Create `scenarios_test.go`
```go
func TestE2E_Basic(t *testing.T) {
    // 1. Build is done in TestMain
    
    // 2. Start Target (Echo Server)
    target, err := NewEchoServer()
    if err != nil {
        t.Fatalf("Failed to start target: %v", err)
    }
    defer target.Close()
    t.Logf("Target listening on %s", target.Addr)
    
    // 3. Резервируем порт для SOCKS сервера
    // RevSocks не выводит реальный порт при :0, поэтому используем фиксированный
    socksPort := GetFreePort(t) // Helper функция
    socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
    
    // 4. Start RevSocks Server
    server := NewProcess(GlobalCtx.BinPath, "server")
    if err := server.Start("-listen", socksAddr, "-pass", "test123"); err != nil {
        t.Fatalf("Failed to start server: %v", err)
    }
    defer server.Stop()
    
    // Ждём готовности сервера
    if err := server.WaitForLog("Listening", 5*time.Second); err != nil {
        t.Fatalf("Server didn't start: %v\nLogs:\n%s", err, server.GetOutput())
    }
    
    // 5. Start RevSocks Client
    client := NewProcess(GlobalCtx.BinPath, "client")
    if err := client.Start("-connect", socksAddr, "-pass", "test123", "-socks", "127.0.0.1:1080"); err != nil {
        t.Fatalf("Failed to start client: %v", err)
    }
    defer client.Stop()
    
    // Ждём подключения клиента
    if err := client.WaitForLog("Connected", 5*time.Second); err != nil {
        t.Fatalf("Client didn't connect: %v\nLogs:\n%s", err, client.GetOutput())
    }
    
    // 6. Test Traffic через SOCKS прокси
    testData := []byte("Hello, RevSocks E2E!")
    if err := TestProxyConnection("127.0.0.1:1080", target.Addr, testData); err != nil {
        t.Fatalf("Proxy test failed: %v", err)
    }
    
    t.Log("✅ E2E test passed")
}

// GetFreePort возвращает свободный порт
func GetFreePort(t *testing.T) int {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to get free port: %v", err)
    }
    port := ln.Addr().(*net.TCPAddr).Port
    ln.Close()
    return port
}
```

## D. Verification
*   **Automated:** Запуск всех тестов.

## E. Local Checklist
```yaml
todos:
  - id: impl-traffic-logic
    content: Реализовать SOCKS5 чекер
    status: pending
  - id: impl-basic-test
    content: Написать TestE2E_Basic
    status: pending
```

## F. Next Action
Prompt:
```text
Все файлы плана созданы. Переходи к реализации.
Начни с шага 01: Setup Directory и main_test.go.
```
