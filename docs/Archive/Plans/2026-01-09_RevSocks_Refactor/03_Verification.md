# Step 3: Verification & Testing

## A. Context Setup
**Target Files:**
- `revsocks/tests/e2e/` (Scenarios)
- `revsocks/cmd/agent/`
- `revsocks/cmd/server/`

## B. Verification Strategy

### 1. Compilation Check
- Проверка сборки `go build ./cmd/agent`.
- Проверка сборки `go build ./cmd/server`.
- Проверка `make stealth`.

### 2. Functional Test (Manual)
1. Запустить сервер: `./revsocks-server -listen :10443 -pass test`.
2. Запустить агент: `./revsocks-agent -connect 127.0.0.1:10443 -pass test`.
3. Подключиться SOCKS5 клиентом: `curl --socks5 127.0.0.1:1080 http://ifconfig.me`.

### 3. Stealth Build Test
1. Настроить `config.yaml` (local test).
2. `make stealth`.
3. Запустить полученный бинарник.
4. Проверить подключение.

### 4. Automated E2E
- Запустить `go test ./tests/e2e/...`.
- Убедиться, что тесты проходят с новой структурой бинарников.

## C. Implementation Steps

### 1. Fix Tests Imports
- Обновить импорты в `*_test.go` файлах (так как пакеты переместились).
- Возможно, придется перенести тесты в соответствующие папки (`internal/agent/client_test.go`).

### 2. Run Tests
- Выполнение тестов и исправление ошибок компиляции тестов.

## D. Verification
- **Success Criteria**: Все тесты проходят, бинарники собираются, функционал работает.

## E. Local Checklist
todos:
  - id: verify-compile
    content: Проверка компиляции всех целей
    status: pending
  - id: verify-manual
    content: Ручной тест связки Client-Server
    status: pending
  - id: verify-stealth
    content: Тест stealth сборки
    status: pending
  - id: verify-e2e
    content: Прогон E2E тестов
    status: pending
