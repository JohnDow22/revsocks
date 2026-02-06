# Step 1: Structure Migration & Code Separation

## A. Context Setup
**Target Files:**
- `revsocks/main.go` (Split -> `cmd/agent/main.go`, `cmd/server/main.go`)
- `revsocks/rclient.go` (Move -> `internal/agent/`)
- `revsocks/rserver.go` (Move -> `internal/server/`)
- `revsocks/yamux_config.go` (Move -> `internal/transport/`)
- `revsocks/tlshelp.go` (Move -> `internal/transport/`)
- `revsocks/version.go` (Move -> `internal/common/`)

**Reference Files:**
- `revsocks/go.mod`
- `revsocks/config.yaml`

## B. Detailed Design

### Новая Структура
```text
revsocks/
├── cmd/
│   ├── agent/
│   │   └── main.go       # Entry point для агента (Flags: connect, pass, proxy...)
│   └── server/
│       └── main.go       # Entry point для сервера (Flags: listen, cert, socks...)
├── internal/
│   ├── common/           # Version, Logging helpers
│   │   └── version.go
│   ├── transport/        # Network layer
│   │   ├── yamux.go      # yamux_config.go
│   │   └── tls.go        # tlshelp.go
│   ├── agent/            # Core business logic for Client
│   │   └── client.go     # rclient.go logic
│   └── server/           # Core business logic for Server
│       ├── server.go     # rserver.go logic
│       └── dns.go        # rdns.go logic
```

### Логика разделения
1. **cmd/agent/main.go**:
    - Инициализирует флаги, специфичные для агента (`connect`, `pass`, `proxy`, `tls`).
    - Вызывает `agent.Run()`.
    - Обрабатывает Graceful Shutdown.
2. **cmd/server/main.go**:
    - Инициализирует флаги сервера (`listen`, `cert`, `socks`).
    - Вызывает `server.Run()`.
3. **internal/transport**:
    - Содержит конфигурацию Yamux и TLS хелперы, используемые обеими сторонами.

## C. Implementation Steps

### 1. Подготовка директорий
- Создать структуру папок `cmd/agent`, `cmd/server`, `internal/common`, `internal/transport`, `internal/agent`, `internal/server`.

### 2. Перенос Shared Code
- Переместить `version.go` -> `internal/common/version.go` (изменить package на `common`).
- Переместить `yamux_config.go` -> `internal/transport/yamux.go` (изменить package на `transport`).
- Переместить `tlshelp.go` -> `internal/transport/tls.go` (изменить package на `transport`).

### 3. Перенос Agent Logic
- Переместить `rclient.go` -> `internal/agent/client.go`.
- Рефакторинг:
    - Изменить package на `agent`.
    - Экспортировать основные функции (например, `ConnectForSocks` -> `Run`).
    - Исправить импорты (добавить импорт `internal/transport`, `internal/common`).

### 4. Перенос Server Logic
- Переместить `rserver.go` -> `internal/server/server.go`.
- Переместить `rdns.go` -> `internal/server/dns.go`.
- Рефакторинг:
    - Изменить package на `server`.
    - Исправить импорты.

### 5. Создание Entry Points
- Создать `cmd/agent/main.go`:
    - Скопировать клиентскую часть из старого `main.go`.
    - Настроить вызов `agent` пакета.
- Создать `cmd/server/main.go`:
    - Скопировать серверную часть из старого `main.go`.
    - Настроить вызов `server` пакета.

## D. Verification
- **Manual**: Попытка `go build ./cmd/agent` и `go build ./cmd/server`.
- **Automated**: Запуск unit тестов (потребуется их перемещение/адаптация).

## E. Local Checklist
todos:
  - id: step-1-dirs
    content: Создать структуру директорий
    status: pending
  - id: step-2-shared
    content: Перенос common и transport кода
    status: pending
  - id: step-3-agent-lib
    content: Перенос rclient.go в internal/agent
    status: pending
  - id: step-4-server-lib
    content: Перенос rserver.go в internal/server
    status: pending
  - id: step-5-main-split
    content: Разделение main.go на cmd/agent и cmd/server
    status: pending
