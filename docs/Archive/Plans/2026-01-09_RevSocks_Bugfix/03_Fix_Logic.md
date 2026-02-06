# Этап 03: Исправление логических ошибок (Группа Б1)

```yaml
todos:
  - id: B1-1
    content: "Fix busy-loop в rdns.go при разрыве сессии"
    status: pending
    time_estimate: "30 мин"
  - id: B1-2
    content: "Fix игнорирование ошибок strconv.Atoi"
    status: pending
    time_estimate: "15 мин"
  - id: B1-3
    content: "Fix IPv6 parsing"
    status: pending
    time_estimate: "20 мин"
  - id: B1-4
    content: "Fix race condition в h.sessions"
    status: pending
    time_estimate: "30 мин"
```

---

## Context Setup

### Target Files (для редактирования)
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rdns.go` — busy-loop fix
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/main.go` — strconv.Atoi errors
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rserver.go` — IPv6, sessions race
- `@Linux/MyCustomProjects/RevSocks_my/revsocks/rclient.go` — IPv6

### Reference Files (для контекста)
- `@plans/2026-01-09_RevSocks_Bugfix/00_PLAN_INDEX.md` — общий план
- `@4_final.md` — полный список проблем

---

## B1-1: Fix Busy-Loop в rdns.go

### Проблема
Файл: `rdns.go`, строки 29-49
```go
for {
    session, err := dt.DnsClient()
    // ...
    for {
        stream, err := session.Accept()
        if err != nil {
            break  // ← Выход только во внутренний цикл!
        }
    }
    // ← Сразу пытается создать новую сессию без паузы
}
```

При разрыве сессии внутренний `break` выводит во внешний цикл, который мгновенно пытается создать новую — busy-loop.

### Решение
1. Проверять `session.IsClosed()` перед Accept
2. Добавить backoff перед reconnect

### Implementation Steps

**Шаг 1**: Добавить проверку IsClosed в accept loop
```go
// rdns.go, строка ~36
for {
    // Проверяем состояние сессии перед Accept
    if session.IsClosed() {
        log.Println("DNS session closed, reconnecting...")
        break
    }
    stream, err := session.Accept()
    if err != nil {
        if session.IsClosed() {
            log.Println("DNS session closed during accept")
            break
        }
        log.Printf("Error accepting stream: %v", err)
        continue  // Не break — пробуем снова
    }
    // ...
}
```

**Шаг 2**: Добавить delay перед reconnect
```go
// rdns.go, после внутреннего for
log.Println("DNS session ended, waiting before reconnect...")
time.Sleep(5 * time.Second)  // Backoff
```

---

## B1-2: Fix игнорирование ошибок strconv.Atoi

### Проблема
Файл: `main.go`, строки 131-133, 148-150
```go
opttimeout, _ := strconv.Atoi(CurOptions.optproxytimeout)
// ← Ошибка игнорируется, opttimeout = 0
```

Если пользователь введёт "100ms" вместо "100" — таймаут будет 0.

### Решение
Валидировать входные данные с понятным сообщением об ошибке.

### Implementation Steps

**Шаг 1**: Добавить валидацию в main.go
```go
// main.go, строка ~131
if CurOptions.optproxytimeout != "" {
    opttimeout, err := strconv.Atoi(CurOptions.optproxytimeout)
    if err != nil {
        log.Fatalf("Invalid proxytimeout value '%s': must be integer (milliseconds)", CurOptions.optproxytimeout)
    }
    if opttimeout <= 0 {
        log.Fatalf("Invalid proxytimeout value: must be positive integer")
    }
    proxytout = time.Millisecond * time.Duration(opttimeout)
}
```

**Шаг 2**: Аналогично для второго места (строка ~148)

---

## B1-3: Fix IPv6 Parsing

### Проблема
Файл: `rserver.go`, строка 345
```go
var listenstr = strings.Split(clients, ":")
// clients = "[::1]:1080" → listenstr = ["[", "", "1]", "1080"]
```

`strings.Split` ломает IPv6 адреса.

### Решение
Использовать `net.SplitHostPort()` — правильно обрабатывает IPv6.

### Implementation Steps

**Шаг 1**: Заменить Split на SplitHostPort
```go
// rserver.go, строка ~345
host, portStr, err := net.SplitHostPort(clients)
if err != nil {
    log.Fatalf("Invalid client listen address '%s': %v", clients, err)
}
portnum, err := strconv.Atoi(portStr)
if err != nil {
    log.Fatalf("Invalid port in '%s': %v", clients, err)
}
```

**Шаг 2**: Аналогично в `listenForWebsocketAgents` (строка ~238)

**Шаг 3**: Проверить `rclient.go` на аналогичные проблемы

---

## B1-4: Fix Race Condition в h.sessions

### Проблема
Файл: `rserver.go`
Слайс `h.sessions` в `agentHandler.ServeHTTP` больше не используется (удалён в v2.3), но если он вернётся — race condition.

### Анализ текущего кода
```go
// rserver.go:159
type agentHandler struct {
    mu        sync.Mutex
    listenstr string
    portnext  int
    timeout   time.Duration
}
```

✅ **Слайс sessions удалён** — проблема уже решена в прошлых итерациях.
✅ **SessionManager** использует `sync.RWMutex` — корректно.

### Действие
Проверить, что нигде не осталось старого кода с `h.sessions`.

```bash
grep -n "h\.sessions" rserver.go
# Должен вернуть пустой результат
```

---

## Verification

### Manual Testing
```bash
# 1. Запуск сервера с IPv6
./revsocks -listen "[::1]:8443" -socks "[::1]:1080" -pass test -tls

# 2. Клиент с неправильным таймаутом (должен быть fatal)
./revsocks -connect server:8443 -pass test -proxytimeout "abc"
# Expected: "Invalid proxytimeout value 'abc': must be integer (milliseconds)"

# 3. DNS reconnect (симуляция разрыва)
# Запустить DNS сервер, убить сессию, проверить что нет busy-loop в логах
```

### Automated Testing
```bash
# Unit test для strconv валидации
go test -run TestProxyTimeoutValidation ./...

# Проверка IPv6 парсинга
go test -run TestIPv6AddressParsing ./...
```

---

## Acceptance Criteria

- [ ] `rdns.go`: Accept loop выходит при session.IsClosed() без busy-loop
- [ ] `main.go`: Невалидный proxytimeout вызывает log.Fatalf с понятным сообщением
- [ ] `rserver.go`: IPv6 адреса "[::1]:1080" корректно парсятся
- [ ] `rserver.go`: Нет остаточного кода с h.sessions слайсом
- [ ] Все изменения не ломают существующий функционал

---

## Next Action

После завершения этого этапа:
```
@plans/2026-01-09_RevSocks_Bugfix/05_Testing.md
```
