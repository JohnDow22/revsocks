# Step 4: Testing Strategy & Documentation

## ⚠️ ОБНОВЛЕНО: Учтён рефакторинг (2026-01-09)
Используем новую структуру тестов в `internal/` и существующий E2E framework в `tests/e2e/`.

## A. Context Setup

### Target Files
- `internal/server/agent_manager_test.go` (New)
- `internal/agent/client_test.go` (Modify/New)
- `internal/server/api_test.go` (New)
- `tests/e2e/scenarios_test.go` (Modify - добавить beacon scenarios)

### Reference Files
- `tests/e2e/README.md` (существующий E2E framework)
- `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc`

## B. Detailed Design

### 1. Unit Testing (Go)
Use standard `testing` package.

**`agent_manager_test.go`:**
- `TestLoadSave`: Create temp file, save config, load it back, compare.
- `TestThreadSafety`: Run concurrent `RegisterAgent` calls.

**`rclient_test.go`:**
- `TestJitter`: Run `calculateJitter(100, 10)` 1000 times. Assert min >= 90, max <= 110.

### 2. Integration Testing (E2E)
Использовать существующий E2E framework из `tests/e2e/`.

**Scenario: Sleep Cycle** (добавить в `tests/e2e/scenarios_test.go`)
1.  Start Server с AgentManager и default config (все агенты в SLEEP 5s).
2.  Start Agent.
3.  **Assert:** Agent подключается, получает `CMD SLEEP 5 10`, отключается.
4.  **Assert:** После ~5s агент переподключается.
5.  Update agent config на сервере (через AgentManager API) → TUNNEL mode.
6.  **Assert:** При следующем check-in агент получает `CMD TUNNEL`.
7.  **Assert:** Yamux session устанавливается, SOCKS5 работает.

**Scenario: Tunnel to Sleep Transition**
1.  Агент в TUNNEL режиме, активная yamux сессия.
2.  Админ меняет режим на SLEEP через API.
3.  Текущая сессия продолжает работать (не прерывается).
4.  При следующем переподключении агент уходит в SLEEP.

### 3. API Testing
**Automated (curl/go tests):**
- `GET /api/agents` → проверка формата JSON
- `POST /api/agents/{id}/config` → изменение режима, проверка persistence
- Invalid token → HTTP 401
- Invalid agent ID → HTTP 404

## C. Implementation Steps

1.  **Write `internal/server/agent_manager_test.go`:**
    - `TestLoadSave`: persistence
    - `TestThreadSafety`: concurrent RegisterAgent
    - `TestUpdateConfig`: изменение режима

2.  **Write `internal/agent/client_test.go`:**
    - `TestJitter`: статистическое распределение
    - `TestIDGeneration`: persistence ID файла

3.  **Write `internal/server/api_test.go`:**
    - `TestAPIAuth`: проверка токена
    - `TestListAgents`: формат ответа
    - `TestUpdateAgent`: изменение конфига

4.  **Update `tests/e2e/scenarios_test.go`:**
    - Добавить `TestBeaconSleepCycle`
    - Добавить `TestTunnelToSleepTransition`

5.  **Update Documentation:**
    - `README.md`: добавить раздел про Beaconing mode
    - `feature.md`: отметить выполненные пункты
    - Создать `docs/04_Features/BEACON_MODE.md` с описанием

## D. Verification
1.  Run unit tests: `cd internal/server && go test -v`
2.  Run unit tests: `cd internal/agent && go test -v`
3.  Run E2E tests: `cd tests/e2e && go test -v -run TestBeacon`
4.  Manual verification: запустить server + agent, проверить beacon поведение

## E. Local Checklist
```yaml
todos:
  - id: unit-tests-manager
    content: Реализовать unit tests для AgentManager (internal/server/)
    status: pending
  - id: unit-tests-client
    content: Реализовать unit tests для Jitter/ID (internal/agent/)
    status: pending
  - id: unit-tests-api
    content: Реализовать unit tests для HTTP API (internal/server/)
    status: pending
  - id: e2e-beacon
    content: Реализовать E2E тесты для beacon mode (tests/e2e/)
    status: pending
  - id: docs-update
    content: Обновить документацию (README, feature.md, создать BEACON_MODE.md)
    status: pending
```
