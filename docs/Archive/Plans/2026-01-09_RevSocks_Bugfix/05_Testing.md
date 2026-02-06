# Этап 05: Стратегия тестирования

```yaml
todos:
  - id: T-1
    content: "Создать структуру тестов"
    status: pending
    time_estimate: "30 мин"
  - id: T-2
    content: "Unit тесты парсеров и валидаторов"
    status: pending
    time_estimate: "1 час"
  - id: T-3
    content: "Unit тесты SessionManager"
    status: pending
    time_estimate: "1 час"
  - id: T-4
    content: "Integration тест handshake протокола"
    status: pending
    time_estimate: "1.5 часа"
  - id: T-5
    content: "Integration тест reconnect сценария"
    status: pending
    time_estimate: "1 час"
```

---

## Context Setup

### Target Files (для создания)
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/main_test.go`
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rserver_test.go`
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rclient_test.go`
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/protocol_test.go`

### Reference Files (для контекста)
- `@.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc`
- `@plans/2026-01-09_RevSocks_Bugfix/00_PLAN_INDEX.md`

---

## Уровень тестирования

Согласно Testing Decision Matrix:
- **Размер проекта**: ~1500 LOC → Level 2 (Integration)
- **Критичность**: Высокая (сетевой протокол, безопасность)
- **Рекомендация**: Unit + Integration тесты

---

## T-1: Структура тестов

### Файловая структура
```
revsocks/
├── main.go
├── main_test.go        # Тесты валидаторов CLI
├── rserver.go
├── rserver_test.go     # Тесты SessionManager, extractAgentIP
├── rclient.go
├── rclient_test.go     # Тесты getAgentID, sanitizeProxyConnect
├── protocol_test.go    # Integration тесты handshake
└── testdata/           # Тестовые данные если нужны
```

### Базовый шаблон теста
```go
package main

import (
    "testing"
)

func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid case", "input", "expected", false},
        {"edge case", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

---

## T-2: Unit тесты парсеров и валидаторов

### main_test.go

```go
package main

import "testing"

func TestParseProxyAuth(t *testing.T) {
    tests := []struct {
        name       string
        input      string
        wantDomain string
        wantUser   string
        wantPass   string
        wantErr    bool
    }{
        // Валидные случаи
        {"domain/user:pass", "CORP/admin:secret123", "CORP", "admin", "secret123", false},
        {"user:pass", "admin:secret123", "", "admin", "secret123", false},
        {"empty", "", "", "", "", false},
        
        // Edge cases
        {"pass with colon", "user:pass:word", "", "user", "pass:word", false},
        {"domain with slash", "CORP/DEPT/user:pass", "CORP", "DEPT/user", "pass", false},
        
        // Ошибки
        {"no password", "admin", "", "", "", true},
        {"only domain", "CORP/admin", "", "", "", true},
        {"domain no pass", "CORP/", "", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            domain, user, pass, err := parseProxyAuth(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr {
                if domain != tt.wantDomain {
                    t.Errorf("domain = %v, want %v", domain, tt.wantDomain)
                }
                if user != tt.wantUser {
                    t.Errorf("user = %v, want %v", user, tt.wantUser)
                }
                if pass != tt.wantPass {
                    t.Errorf("pass = %v, want %v", pass, tt.wantPass)
                }
            }
        })
    }
}
```

---

## T-3: Unit тесты SessionManager

### rserver_test.go

```go
package main

import (
    "context"
    "sync"
    "testing"
)

func TestExtractAgentIP(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"IPv4 with port", "192.168.1.1:8080", "192.168.1.1"},
        {"IPv6 with port", "[::1]:8080", "::1"},
        {"IPv6 full", "[2001:db8::1]:443", "2001:db8::1"},
        {"no port", "192.168.1.1", "192.168.1.1"},
        {"empty", "", ""},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := extractAgentIP(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}

func TestSessionManagerRace(t *testing.T) {
    sm := &SessionManager{
        sessions:  make(map[string]*ManagedSession),
        portCache: make(map[string]int),
    }
    
    var wg sync.WaitGroup
    agentIDs := []string{"agent1", "agent2", "agent3"}
    
    // Параллельная регистрация
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            agentID := agentIDs[idx%3]
            _, cancel := context.WithCancel(context.Background())
            sm.RegisterSession(agentID, nil, 1080+idx, cancel)
        }(i)
    }
    
    // Параллельное удаление
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            agentID := agentIDs[idx%3]
            sm.UnregisterSession(agentID, uint64(idx))
        }(i)
    }
    
    wg.Wait()
    
    // Проверяем что не было паники
    count := sm.GetSessionCount()
    t.Logf("Final session count: %d", count)
}

func TestSessionManagerGenerationProtection(t *testing.T) {
    sm := &SessionManager{
        sessions:  make(map[string]*ManagedSession),
        portCache: make(map[string]int),
    }
    
    // Регистрируем сессию
    _, cancel1 := context.WithCancel(context.Background())
    gen1, _ := sm.RegisterSession("agent1", nil, 1080, cancel1)
    
    // Регистрируем новую сессию с тем же ID
    _, cancel2 := context.WithCancel(context.Background())
    gen2, _ := sm.RegisterSession("agent1", nil, 1081, cancel2)
    
    // Пытаемся удалить со старым generation
    sm.UnregisterSession("agent1", gen1)
    
    // Сессия должна остаться (защита generation)
    if sm.GetSessionCount() != 1 {
        t.Errorf("Session should not be deleted with old generation")
    }
    
    // Удаляем с правильным generation
    sm.UnregisterSession("agent1", gen2)
    
    if sm.GetSessionCount() != 0 {
        t.Errorf("Session should be deleted with correct generation")
    }
}
```

---

## T-4: Integration тест handshake протокола

### protocol_test.go

```go
package main

import (
    "bytes"
    "io"
    "net"
    "testing"
    "time"
)

func TestHandshakeProtocolV2(t *testing.T) {
    // Создаём pipe для имитации TCP
    client, server := net.Pipe()
    defer client.Close()
    defer server.Close()
    
    password := "testpassword"
    agentID := "test-agent-123"
    
    // Клиент отправляет handshake
    go func() {
        // Password padded до 64 байт
        paddedPassword := make([]byte, 64)
        copy(paddedPassword, []byte(password))
        
        // Length-prefixed agentID
        handshake := append(paddedPassword, byte(len(agentID)))
        handshake = append(handshake, []byte(agentID)...)
        
        client.Write(handshake)
        
        // Ждём ACK
        ack := make([]byte, 2)
        client.SetReadDeadline(time.Now().Add(5 * time.Second))
        n, _ := io.ReadFull(client, ack)
        
        if string(ack[:n]) != "OK" {
            t.Errorf("Expected OK, got %s", string(ack[:n]))
        }
    }()
    
    // Сервер читает handshake
    buf := make([]byte, 64)
    io.ReadFull(server, buf)
    receivedPassword := string(bytes.TrimRight(buf, "\x00"))
    
    if receivedPassword != password {
        t.Errorf("Password mismatch: got %s, want %s", receivedPassword, password)
    }
    
    // Читаем length
    lengthBuf := make([]byte, 1)
    io.ReadFull(server, lengthBuf)
    agentIDLen := int(lengthBuf[0])
    
    // Читаем agentID
    agentIDBuf := make([]byte, agentIDLen)
    io.ReadFull(server, agentIDBuf)
    receivedAgentID := string(agentIDBuf)
    
    if receivedAgentID != agentID {
        t.Errorf("AgentID mismatch: got %s, want %s", receivedAgentID, agentID)
    }
    
    // Отправляем ACK
    server.Write([]byte("OK"))
    
    time.Sleep(100 * time.Millisecond) // Даём горутине завершиться
}

func TestHandshakeWrongPassword(t *testing.T) {
    client, server := net.Pipe()
    defer client.Close()
    defer server.Close()
    
    correctPassword := "correct"
    wrongPassword := "wrong"
    
    // Клиент отправляет неправильный пароль
    go func() {
        paddedPassword := make([]byte, 64)
        copy(paddedPassword, []byte(wrongPassword))
        client.Write(paddedPassword)
        client.Write([]byte{0}) // Пустой agentID
        
        // Ждём NACK
        nack := make([]byte, 2)
        client.SetReadDeadline(time.Now().Add(5 * time.Second))
        n, _ := io.ReadFull(client, nack)
        
        if string(nack[:n]) != "NO" {
            t.Errorf("Expected NO, got %s", string(nack[:n]))
        }
    }()
    
    // Сервер проверяет пароль
    buf := make([]byte, 64)
    io.ReadFull(server, buf)
    receivedPassword := string(bytes.TrimRight(buf, "\x00"))
    
    if receivedPassword == correctPassword {
        server.Write([]byte("OK"))
    } else {
        server.Write([]byte("NO"))
    }
    
    time.Sleep(100 * time.Millisecond)
}
```

---

## T-5: Integration тест reconnect сценария

### reconnect_test.go

```go
package main

import (
    "context"
    "testing"
    "time"
)

func TestReconnectDoesNotLeakPorts(t *testing.T) {
    sm := &SessionManager{
        sessions:  make(map[string]*ManagedSession),
        portCache: make(map[string]int),
    }
    
    agentID := "reconnecting-agent"
    
    // Первое подключение
    _, cancel1 := context.WithCancel(context.Background())
    gen1, port1 := sm.RegisterSession(agentID, nil, 1080, cancel1)
    
    // Симулируем разрыв и переподключение
    time.Sleep(10 * time.Millisecond)
    
    // Второе подключение (должен переиспользовать порт)
    _, cancel2 := context.WithCancel(context.Background())
    gen2, port2 := sm.RegisterSession(agentID, nil, 1081, cancel2)
    
    // Порт должен быть переиспользован из кэша
    if port2 != port1 {
        t.Errorf("Port should be reused: got %d, want %d", port2, port1)
    }
    
    // Generation должен увеличиться
    if gen2 <= gen1 {
        t.Errorf("Generation should increase: gen1=%d, gen2=%d", gen1, gen2)
    }
    
    // Должна быть только одна сессия
    if sm.GetSessionCount() != 1 {
        t.Errorf("Should have 1 session, got %d", sm.GetSessionCount())
    }
    
    // Cleanup
    sm.UnregisterSession(agentID, gen2)
}
```

---

## Запуск тестов

```bash
# Все тесты
cd Linux/MyCustomProjects/RevSocks_my/revsocks
go test -v ./...

# С покрытием
go test -cover ./...

# Race detector
go test -race ./...

# Конкретный тест
go test -v -run TestSessionManagerRace ./...
```

---

## Acceptance Criteria

- [ ] Все тесты проходят: `go test ./...`
- [ ] Нет race conditions: `go test -race ./...`
- [ ] Покрытие критичных функций > 60%
- [ ] Тесты документируют ожидаемое поведение
- [ ] Нет flaky тестов (запуск 10 раз подряд)

---

## Anti-patterns (чего НЕ делать в тестах)

1. **НЕ** тестировать приватные детали реализации
2. **НЕ** использовать time.Sleep для синхронизации (используй channels)
3. **НЕ** игнорировать race detector warnings
4. **НЕ** писать тесты которые зависят от порядка выполнения
5. **НЕ** хардкодить порты (могут быть заняты)

---

## Next Action

После завершения тестирования:
1. Запустить `go test -race -cover ./...`
2. Исправить найденные проблемы
3. Обновить CHANGELOG.md
