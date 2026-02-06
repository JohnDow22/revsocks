# Этап 1: Фикс паник (Crash Prevention)

## Local Checklist

```yaml
todos:
  - id: panic-1
    content: "Валидация ProxyAuth строки"
    status: pending
  - id: panic-2  
    content: "Проверка nil после net.Dial"
    status: pending
  - id: panic-3
    content: "Валидация длины пароля"
    status: pending
```

---

## Context Setup

### Target Files (редактирование)
- `@revsocks/main.go` — строки 127-141 (ProxyAuth parsing)
- `@revsocks/rclient.go` — строки 290-300 (connectviaproxy)
- `@revsocks/rserver.go` — строки 361-366 (password validation)

### Reference Files (контекст)
- `@revsocks/config.yaml.example` — формат proxyauth

---

## Bug #1: Парсинг ProxyAuth (main.go:127-141)

### Проблема
```go
// Текущий код — CRASH при неправильном формате
password = strings.Split(strings.Split(CurOptions.proxyauthstring, "/")[1], ":")[1]
```
Форматы вызывающие panic: `user`, `domain/user`, `user:` (пустой пароль)

### Решение
Добавить функцию валидации с проверкой количества частей:

```go
// parseProxyAuth парсит строку формата "domain/user:pass" или "user:pass"
// Возвращает (domain, user, pass, error)
func parseProxyAuth(auth string) (string, string, string, error) {
    if auth == "" {
        return "", "", "", nil
    }
    
    var domain, userPass string
    if strings.Contains(auth, "/") {
        parts := strings.SplitN(auth, "/", 2)
        if len(parts) != 2 {
            return "", "", "", fmt.Errorf("invalid proxyauth format: missing user:pass after domain")
        }
        domain = parts[0]
        userPass = parts[1]
    } else {
        userPass = auth
    }
    
    // Разбираем user:pass
    parts := strings.SplitN(userPass, ":", 2)
    if len(parts) != 2 {
        return "", "", "", fmt.Errorf("invalid proxyauth format: expected user:pass, got %q", userPass)
    }
    
    return domain, parts[0], parts[1], nil
}
```

### Implementation Steps

1. Добавить функцию `parseProxyAuth()` после `type AppOptions struct` (строка ~40)
2. Заменить блок строк 127-141 на:
```go
if CurOptions.proxyauthstring != "" {
    var err error
    domain, username, password, err = parseProxyAuth(CurOptions.proxyauthstring)
    if err != nil {
        log.Fatalf("Proxy auth error: %v", err)
    }
    log.Printf("Using domain %s with user %s", domain, username)
}
```

---

## Bug #2: Nil Pointer после net.Dial (rclient.go:290-300)

### Проблема
```go
conn, err := net.Dial("tcp", proxyaddr)
if err != nil {
    log.Printf("Error connect to %s: %v", proxyaddr, err)
    // НЕТ return! Далее conn.Write() вызывает panic
}
conn.Write(...)  // CRASH если conn == nil
```

### Решение
Добавить `return nil` после логирования ошибки:

```go
conn, err := net.Dial("tcp", proxyaddr)
if err != nil {
    log.Printf("Error connect to %s: %v", proxyaddr, err)
    return nil  // <-- ДОБАВИТЬ
}
```

Также добавить проверку `resp != nil` после `http.ReadResponse`:

```go
resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
if err != nil || resp == nil {
    log.Printf("Error reading proxy response: %v", err)
    conn.Close()
    return nil
}
status := resp.Status
```

### Implementation Steps

1. Строка 294: добавить `return nil` после лога ошибки
2. Строки 299-300: заменить на безопасную версию с проверкой resp

---

## Bug #3: Длинный пароль (rserver.go:361-366)

### Проблема
```go
statusb := make([]byte, 64)
_, _ = io.ReadFull(reader, statusb)
if string(statusb)[:len(agentpassword)] != agentpassword  // CRASH если password > 64
```

### Решение
Валидировать длину пароля при старте сервера:

```go
// В listenForAgents, после инициализации
if len(agentpassword) > 64 {
    return fmt.Errorf("password too long: max 64 bytes, got %d", len(agentpassword))
}
```

И добавить защиту при сравнении:

```go
pwdLen := len(agentpassword)
if pwdLen > 64 {
    pwdLen = 64
}
if pwdLen > len(statusb) || string(statusb)[:pwdLen] != agentpassword[:pwdLen] {
    // invalid password
}
```

### Implementation Steps

1. Строка 305 (начало `listenForAgents`): добавить валидацию длины пароля
2. Строка 366: заменить на безопасное сравнение

---

## Verification

### Manual Testing
1. Запустить клиент с невалидным `-proxyauth user` → ожидаем graceful error, не crash
2. Запустить клиент с недоступным прокси → ожидаем reconnect, не crash
3. Запустить сервер с паролем > 64 символов → ожидаем error при старте

### Automated Testing
```bash
# Тест 1: невалидный proxyauth
./revsocks -connect test:8080 -proxyauth "useronly" -pass test 2>&1 | grep -i "error"

# Тест 2: недоступный прокси
./revsocks -connect test:8080 -proxy 127.0.0.1:9999 -pass test 2>&1 | grep -i "error"
```

---

## Next Action

После фикса всех 3 паник → переход к `02_Fix_Leaks.md`

Промпт для следующего чата:
```
Роль: Go разработчик.
Задача: Реализовать фиксы паник согласно 01_Fix_Panics.md
Target Files: main.go:127-141, rclient.go:290-300, rserver.go:361-366
Критерии: Нет panic при некорректных входных данных
```
