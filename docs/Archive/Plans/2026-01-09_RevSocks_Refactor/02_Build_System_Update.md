# Step 2: Build System & Script Adaptation

## A. Context Setup
**Target Files:**
- `revsocks/Makefile`
- `revsocks/build_stealth.sh`
- `revsocks/tests/e2e/builder.go` (Test framework builder)

**Reference Files:**
- `revsocks/config.yaml`
- `plans/2026-01-09_RevSocks_Refactor/01_Structure_Migration.md`

## B. Detailed Design

### Makefile
Необходимо обновить цели сборки:
- `default`: собирает оба бинарника.
- `agent`: `go build -o revsocks-agent ./cmd/agent`.
- `server`: `go build -o revsocks-server ./cmd/server`.
- `stealth`: вызов обновленного `build_stealth.sh`.

### build_stealth.sh
**КРИТИЧЕСКИЙ МОМЕНТ:** Скрипт использует `sed` для инъекции переменных в Go код.
Сейчас он ищет переменные в `main.go`.
После рефакторинга переменные конфигурации (IP, Password, Failover) будут находиться в `cmd/agent/main.go`.

**Необходимые изменения:**
1. Путь к исходнику: `main.go` -> `cmd/agent/main.go`.
2. Путь к `rclient.go` -> `internal/agent/client.go` (для инъекции AgentID и SocksAuth).
3. Путь к `yamux_config.go` -> `internal/transport/yamux.go` (для тюнинга таймингов).
4. Backup/Restore логика должна учитывать новые пути.

### E2E Builder (`tests/e2e/builder.go`)
Тестовый фреймворк собирает бинарники перед тестом.
Нужно научить его собирать `cmd/agent` и `cmd/server` вместо одного `main.go`.

## C. Implementation Steps

### 1. Обновление Makefile
- Добавить цели `build-agent` и `build-server`.
- Обновить `clean` для удаления новых бинарников.

### 2. Рефакторинг build_stealth.sh
- Обновить переменную `SRC_FILE="cmd/agent/main.go"`.
- Обновить пути для backup файлов.
- Проверить все `sed` выражения на соответствие новому коду в `cmd/agent/main.go`.
- Убедиться, что инъекция `Failover Configuration` (массив серверов) попадает в правильное место в новом `main.go`.

### 3. Обновление E2E Builder
- Модифицировать функцию сборки в `tests/e2e/builder.go`.
- Указать новые пути к `main` пакетам.

## D. Verification
- **Manual**: Запуск `make stealth` и проверка, что бинарник создается и работает.
- **Automated**: Запуск `make stealth-test` (если применимо).

## E. Local Checklist
todos:
  - id: fix-makefile
    content: Обновить Makefile
    status: pending
  - id: fix-stealth-script
    content: Адаптировать build_stealth.sh под новую структуру
    status: pending
  - id: fix-e2e-builder
    content: Обновить tests/e2e/builder.go
    status: pending
