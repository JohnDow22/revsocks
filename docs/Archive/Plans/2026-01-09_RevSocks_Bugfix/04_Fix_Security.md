# Этап 04: Исправление протокола и синхронизации (Группа А1)

```yaml
todos:
  - id: A1-1
    content: "Добавить length-prefixed AgentID в протокол"
    status: pending
    time_estimate: "1 час"
  - id: A1-2
    content: "Заменить time.Sleep на ACK handshake"
    status: pending
    time_estimate: "1.5 часа"
  - id: A1-3
    content: "Добавить подтверждение авторизации (OK/FAIL)"
    status: pending
    time_estimate: "1 час"
```

---

## Context Setup

### Target Files (для редактирования)
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rserver.go` — серверная часть протокола
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rclient.go` — клиентская часть протокола

### Reference Files (для контекста)
- `@plans/2026-01-09_RevSocks_Bugfix/00_PLAN_INDEX.md` — общий план
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/yamux_config.go` — общая конфигурация

---

## Текущий протокол (v1)

```
Client → Server:
  [password (64 bytes max)]['\n'][agentID (variable)]
  
Server → Client:
  (ничего, клиент ждёт 1 сек и начинает yamux)
```

**Проблемы**:
1. Один `Read()` может вернуть часть agentID при фрагментации
2. `time.Sleep(1s)` не гарантирует синхронизацию
3. Клиент не знает, принят ли пароль

---

## Новый протокол (v2)

```
Client → Server:
  [password (64 bytes padded)]
  [agentID length (1 byte)]
  [agentID (0-255 bytes)]

Server → Client:
  [status (2 bytes)]: "OK" или "NO"
  
Client:
  if "OK" → start yamux
  if "NO" или timeout → reconnect
```

**Преимущества**:
- Length-prefixed = надёжное чтение
- ACK = синхронизация без Sleep
- Backward compatible: старый клиент получит timeout

---

## A1-1: Length-prefixed AgentID

### Implementation Steps

**Шаг 1**: Добавить константы протокола
```go
// rserver.go, после импортов
const (
    ProtocolVersion   = 2
    PasswordSize      = 64
    MaxAgentIDLength  = 255
    HandshakeACK      = "OK"
    HandshakeNACK     = "NO"
)
```

**Шаг 2**: Переписать чтение AgentID на сервере
Файл: `rserver.go`, заменить строки ~398-424

```go
// Читаем agentID length (1 байт)
lengthBuf := make([]byte, 1)
conn.SetReadDeadline(time.Now().Add(2 * time.Second))
_, err := io.ReadFull(reader, lengthBuf)
if err != nil {
    log.Printf("[%s] Error reading agentID length: %v", agentstr, err)
    agentID = extractAgentIP(agentstr) // fallback
} else {
    agentIDLen := int(lengthBuf[0])
    if agentIDLen > 0 && agentIDLen <= MaxAgentIDLength {
        agentIDBuf := make([]byte, agentIDLen)
        _, err = io.ReadFull(reader, agentIDBuf)
        if err != nil {
            log.Printf("[%s] Error reading agentID: %v", agentstr, err)
            agentID = extractAgentIP(agentstr)
        } else {
            agentID = string(agentIDBuf)
        }
    } else {
        agentID = extractAgentIP(agentstr)
    }
}
```

**Шаг 3**: Переписать отправку AgentID на клиенте
Файл: `rclient.go`, заменить строки ~500-506

```go
// Формируем handshake v2: password (padded) + length + agentID
currentAgentID := getAgentID()
if len(currentAgentID) > MaxAgentIDLength {
    currentAgentID = currentAgentID[:MaxAgentIDLength]
}

// Password padding до 64 байт
paddedPassword := make([]byte, PasswordSize)
copy(paddedPassword, []byte(agentpassword))

// Length-prefixed agentID
handshake := append(paddedPassword, byte(len(currentAgentID)))
handshake = append(handshake, []byte(currentAgentID)...)

log.Printf("Using agent ID: %s", currentAgentID)
```

---

## A1-2: ACK Handshake вместо Sleep

### Implementation Steps

**Шаг 1**: Сервер отправляет ACK после валидации
Файл: `rserver.go`, после проверки пароля (~строка 393)

```go
// Отправляем ACK клиенту
_, err := conn.Write([]byte(HandshakeACK))
if err != nil {
    log.Printf("[%s] Error sending ACK: %v", agentstr, err)
    conn.Close()
    continue
}
log.Printf("[%s] Sent ACK, starting yamux", agentID)
```

**Шаг 2**: Сервер отправляет NACK при неверном пароле
Файл: `rserver.go`, в блоке else (~строка 376)

```go
// Неверный пароль или HTTP запрос
if strings.Contains(status, " HTTP/1.1") {
    // HTTP redirect как раньше
} else {
    // Отправляем NACK для не-HTTP клиентов
    conn.Write([]byte(HandshakeNACK))
    conn.Close()
}
```

**Шаг 3**: Клиент ждёт ACK вместо Sleep
Файл: `rclient.go`, заменить `time.Sleep(time.Second * 1)` (~строка 513)

```go
// Ждём ACK от сервера
ackBuf := make([]byte, 2)
conn.SetReadDeadline(time.Now().Add(5 * time.Second))
n, err := io.ReadFull(conn, ackBuf)
if err != nil || string(ackBuf[:n]) != HandshakeACK {
    log.Printf("Handshake failed: %v (response: %s)", err, string(ackBuf[:n]))
    conn.Close()
    return errors.New("handshake failed")
}
log.Println("Received ACK, starting yamux")

// Сбрасываем deadline для yamux
conn.SetReadDeadline(time.Time{})
```

---

## A1-3: Подтверждение авторизации

Уже реализовано в A1-2 (ACK/NACK).

Дополнительно: добавить логирование причины отказа.

```go
// rserver.go, при NACK
log.Printf("[%s] Authentication failed: password mismatch", agentstr)
```

---

## Backward Compatibility

### Сценарий: Новый клиент + Старый сервер
- Клиент отправляет v2 handshake
- Сервер читает только пароль (первые 64 байта)
- Сервер НЕ шлёт ACK
- Клиент получает timeout → reconnect с fallback

### Сценарий: Старый клиент + Новый сервер
- Клиент отправляет v1 handshake (password + \n + agentID)
- Сервер пытается читать length byte → получает '\n' (0x0A)
- 0x0A = 10 → пытается читать 10 байт agentID
- Может работать случайно, может сломаться

**Рекомендация**: При обновлении обновлять и сервер и клиент.

---

## Verification

### Manual Testing
```bash
# 1. Запуск сервера v2
./revsocks_server -listen :8443 -socks 127.0.0.1:1080 -pass test -tls

# 2. Клиент v2
./revsocks_client -connect server:8443 -pass test -tls
# Expected: "Received ACK, starting yamux"

# 3. Клиент с неправильным паролем
./revsocks_client -connect server:8443 -pass wrong -tls
# Expected: "Handshake failed" + reconnect

# 4. Wireshark/tcpdump
tcpdump -i eth0 port 8443 -X
# Проверить что видно "OK" от сервера
```

### Automated Testing
```bash
go test -run TestHandshakeProtocolV2 ./...
go test -run TestHandshakeTimeout ./...
go test -run TestHandshakeWrongPassword ./...
```

---

## Acceptance Criteria

- [ ] Клиент отправляет length-prefixed agentID
- [ ] Сервер корректно читает agentID любой длины (1-255 байт)
- [ ] Сервер отправляет "OK" при успешной авторизации
- [ ] Сервер отправляет "NO" при неверном пароле
- [ ] Клиент ждёт ACK, не использует Sleep
- [ ] Клиент обрабатывает timeout как ошибку
- [ ] Логи не содержат секретов (пароли замаскированы)

---

## Next Action

После завершения этого этапа:
```
@plans/2026-01-09_RevSocks_Bugfix/05_Testing.md
```
