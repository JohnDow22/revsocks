# Этап 2: Фикс утечек ресурсов (Resource Leaks)

## Local Checklist

```yaml
todos:
  - id: leak-1
    content: "Удаление legacy sessions[] slice"
    status: pending
  - id: leak-2
    content: "Закрытие HTTP Body в WSconnectForSocks"
    status: pending
```

---

## Context Setup

### Target Files (редактирование)
- `@revsocks/rserver.go` — строки 164, 215, 302, 427 (sessions slice)
- `@revsocks/rclient.go` — строки 141-146, 186-189 (HTTP resp.Body)

### Reference Files (контекст)
- `@revsocks/rserver.go:33-141` — SessionManager (уже реализован, это замена для sessions[])

---

## Bug #4: Legacy sessions[] slice не чистится (rserver.go)

### Проблема
Строки где используется legacy slice:
- Строка 164: `sessions []*yamux.Session` в `agentHandler` struct
- Строка 215: `h.sessions = append(h.sessions, session)` в ServeHTTP
- Строка 302: `var sessions []*yamux.Session` в listenForAgents
- Строка 427: `sessions = append(sessions, session)` в listenForAgents

Сессии добавляются, но НИКОГДА не удаляются → бесконечный рост памяти.

### Решение
SessionManager (строки 33-141) уже реализован и используется. Legacy code можно удалить:

1. Удалить поле `sessions` из struct `agentHandler` (строка 164)
2. Удалить `h.sessions = append(...)` (строка 215)
3. Удалить локальную переменную `sessions` (строка 302)
4. Удалить `sessions = append(...)` (строка 427)

### Implementation Steps

1. **rserver.go:164** — удалить строку:
```go
sessions  []*yamux.Session // all sessions (legacy, kept for compatibility)
```

2. **rserver.go:215** — удалить строку:
```go
h.sessions = append(h.sessions, session)
```

3. **rserver.go:302** — удалить строку:
```go
var sessions []*yamux.Session
```

4. **rserver.go:427** — удалить строку:
```go
sessions = append(sessions, session)
```

---

## Bug #5: HTTP Body Leak в WSconnectForSocks (rclient.go)

### Проблема
Строки 141-146:
```go
resp, err := httpClient.Do(req)
if err != nil {
    log.Printf("error making http request to %s: %s\n", wsURL, err)
    return err
}
// resp.Body НИКОГДА не закрывается!
```

При частых 407 Proxy Auth ответах происходит утечка файловых дескрипторов.

### Решение
Добавить `defer resp.Body.Close()` после успешного получения ответа:

```go
resp, err := httpClient.Do(req)
if err != nil {
    log.Printf("error making http request to %s: %s\n", wsURL, err)
    return err
}
defer resp.Body.Close()  // <-- ДОБАВИТЬ
```

### Дополнительно
Проверить все места где используется `httpClient.Do()` или `http.Get()`:
- Строка 141: `httpClient.Do(req)` — нужен defer
- Нет других мест в этом файле

### Implementation Steps

1. **rclient.go:145** — после проверки err добавить:
```go
defer resp.Body.Close()
```

---

## Verification

### Manual Testing
1. Запустить сервер под `top`/`htop`
2. Подключить 10+ клиентов, отключить их
3. Проверить что память не растёт после отключения клиентов

### Automated Testing
```bash
# Мониторинг goroutines (если есть pprof)
curl http://localhost:6060/debug/pprof/goroutine?debug=1 | wc -l

# Проверка file descriptors
ls -la /proc/$(pgrep revsocks)/fd | wc -l
```

---

## Next Action

После фикса утечек → переход к `03_Fix_Logic.md`

Промпт для следующего чата:
```
Роль: Go разработчик.
Задача: Удалить legacy sessions[] и добавить defer resp.Body.Close()
Target Files: rserver.go:164,215,302,427, rclient.go:145
Критерии: Нет роста памяти при переподключениях клиентов
```
