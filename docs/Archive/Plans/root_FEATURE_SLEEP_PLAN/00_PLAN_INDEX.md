# FEATURE_SLEEP_PLAN: Режим Beaconing (Сон) для RevSocks

## ⚠️ ВАЖНО: Рефакторинг завершён (2026-01-09)
План обновлён с учётом выполненного рефакторинга (см. `plans/2026-01-09_RevSocks_Refactor`). 
Новая структура: `cmd/agent/`, `cmd/server/`, `internal/{agent,server,common,transport}`.

## 1. Цель
Внедрить режим "Beaconing" (маякование) для агентов RevSocks, позволяющий им уходить в длительный сон с периодической проверкой задач (Check-in), вместо удержания постоянного TCP-соединения. Это необходимо для скрытности (Stealth) и обхода сетевых детекций.

## 2. Architecture Decision Log

### 2.1 State Reconciliation (Сверка состояний)
**Решение:** Сервер является источником правды (Source of Truth). При каждом подключении (Check-in) агент сообщает свои возможности, а сервер отправляет "Желаемое состояние" (`TUNNEL` или `SLEEP`).
**Обоснование:** Это упрощает агента ("глупый клиент") и централизует логику управления на сервере. Не требует сложных очередей задач (как в Sliver), достаточно синхронного ответа при хендшейке.

### 2.2 Протокол (Handshake v3)
**Решение:** Текстовый протокол поверх TCP перед Yamux.
`Client -> AUTH <password> <agent_id> <version>`
`Server -> CMD TUNNEL | CMD SLEEP <sec> <jitter> | ERR <message>`
**Обоснование:** Легче отлаживать, проще внедрять, чем бинарный протокол.

### 2.3 Persistence (Хранение данных)
**Решение:** JSON файл (`agents.json`) + In-Memory Map с RWMutex на сервере.
**Обоснование:** Простота реализации, отсутствие внешних зависимостей (SQL), достаточно для <1000 агентов.

### 2.4 Новая архитектура (после рефакторинга)
**Результат:** Код разделён на два бинарника (`revsocks-agent`, `revsocks-server`) с общей логикой в `internal/`.
**Преимущества:** 
- Меньший размер агента (~10.8 MB vs 13.4 MB)
- Чистое разделение ответственности
- Упрощение внедрения новых фич

## 3. Матрица Зависимостей

| Модуль | Статус | Влияние |
| :--- | :--- | :--- |
| `internal/server/server.go` | 🟡 Modify | Добавление `AgentManager`, изменение `handleConnection`. |
| `internal/agent/client.go` | 🟡 Modify | Переход от `connectLoop` к `beaconLoop`, парсинг команд. |
| `internal/server/agent_manager.go` | 🔴 New | Управление состоянием агентов, JSON persistence. |
| `internal/server/api.go` | 🔴 New | HTTP API для управления агентами. |
| `cmd/server/main.go` | 🟡 Modify | Инициализация AgentManager, запуск Admin API. |
| `cmd/agent/main.go` | 🟡 Modify | Инициализация beacon loop. |
| `tools/console/` | 🔴 New | Python CLI (Grumble) для управления. |

## 4. Стратегия Тестирования

### Backend (Go)
*   **Unit Tests:** Парсинг команд, логика `AgentManager` (save/load), расчет Jitter.
*   **Integration Tests:** Mock Server + Real Client. Проверка сценариев:
    1.  Connect -> Sleep -> Disconnect -> Wait -> Connect.
    2.  Connect -> Tunnel -> Yamux Session -> Disconnect.
*   **Ref:** `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc` (Level 2).

### UI (Python)
*   **Unit Tests:** API Client wrapper.
*   **Ref:** `.cursor/rules/Dev_2.0/quality/UI/UI_1.5/additional/Testing_Playwright.mdc` (Not applicable for CLI, use `pytest` for logic).

## 5. ROADMAP

- [x] **Step 1:** Server Core Architecture (AgentManager, Persistence). [01_Server_Architecture.md]
- [x] **Step 2:** Client Core Logic (Beacon Loop, Jitter). [02_Client_Architecture.md]
- [x] **Step 3:** Admin API & Console UI. [03_Admin_API_UI.md]
- [x] **Step 4:** Integration Testing & Documentation. [04_Testing_Strategy.md]
- [x] **Step 5:** Phase 5 Complete - Testing & Documentation. [PHASE_5_COMPLETE.md]

**Status:** ✅ 98% COMPLETE (manual testing pending)

## 6. Global Checklist
```yaml
todos:
  - id: srv-agent-manager
    content: Создать internal/server/agent_manager.go с JSON persistence
    status: completed
    time_estimate: 2 часа
    dependencies: []
  - id: srv-handshake
    content: Обновить internal/server/server.go (handleConnection) для Handshake v3
    status: completed
    time_estimate: 2 часа
    dependencies: [srv-agent-manager]
  - id: srv-main-init
    content: Инициализировать AgentManager в cmd/server/main.go
    status: completed
    time_estimate: 30 минут
    dependencies: [srv-agent-manager]
  - id: cli-agent-id
    content: Реализовать persistent Agent ID в internal/agent/client.go
    status: completed
    time_estimate: 1 час
    dependencies: []
  - id: cli-beacon-loop
    content: Реализовать beaconLoop и обработку команд SLEEP/TUNNEL в internal/agent/client.go
    status: completed
    time_estimate: 3 часа
    dependencies: [srv-handshake, cli-agent-id]
  - id: cli-main-init
    content: Обновить cmd/agent/main.go для beacon loop
    status: completed
    time_estimate: 30 минут
    dependencies: [cli-beacon-loop]
  - id: api-server
    content: Создать internal/server/api.go с HTTP endpoints (List, Update, Kill)
    status: completed
    time_estimate: 2 часа
    dependencies: [srv-agent-manager]
  - id: api-main-init
    content: Запустить Admin API Server в cmd/server/main.go
    status: completed
    time_estimate: 30 минут
    dependencies: [api-server]
  - id: console-ui
    content: Написать tools/console/ Python CLI (Grumble) для управления
    status: completed
    time_estimate: 3 часа
    dependencies: [api-server]
  - id: e2e-tests
    content: Обновить tests/e2e для проверки Sleep/Tunnel режимов
    status: completed
    time_estimate: 2 часа
    dependencies: [cli-beacon-loop, api-server]
  - id: documentation
    content: Создать docs/04_Features/BEACON_MODE.md с полным описанием
    status: completed
    time_estimate: 1 час
    dependencies: [e2e-tests]
  - id: manual-testing
    content: Manual testing (сервер + агент + консоль + SOCKS)
    status: pending
    time_estimate: 30 минут
    dependencies: [documentation]
  - id: readme-update
    content: Обновить README.md с секцией про beacon mode
    status: pending
    time_estimate: 10 минут
    dependencies: [manual-testing]
```

**Completed:** 11/13 tasks (85%)  
**Remaining:** Manual testing, README update
