# RevSocks Console E2E Testing Plan

## 1. Context & Goal
**Goal**: Разработать и внедрить автоматизированное E2E (End-to-End) тестирование для консоли администратора `revsocks/tools/console`.
**Business Value**: Обеспечение стабильности CLI инструмента управления, проверка интеграции с реальным API сервера и корректности отображения данных.
**Context**:
- Сервер (`revsocks-server`) и агент (`revsocks-agent`) уже собраны.
- Консоль написана на Python (`cmd2`).
- Необходимо проверить сценарии взаимодействия: CLI -> API -> Server -> Agent.

## 2. Architecture Decision Log
- **Framework**: `pytest` (стандарт проекта).
- **CLI Interaction**: `pexpect` (или `subprocess` с таймаутами) — для имитации интерактивного ввода пользователя и проверки вывода. Выбрано `pexpect` так как `cmd2` ориентирован на интерактивный режим.
- **Environment**: Тесты поднимают *реальный* инстанс сервера и агента локально на динамических портах (fixture management).
- **Isolation**: Каждый тест (или сессия) получает чистое окружение.

## 3. Dependency Matrix
- **Server Binary**: `revsocks-server` (должен быть доступен в PATH или передан через ENV).
- **Agent Binary**: `revsocks-agent` (аналогично).
- **Python Deps**: `pytest`, `pexpect`, `requests` (для проверки состояния API в обход CLI).

## 4. Testing Strategy
Согласно `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc` (Level 2/3):
- **E2E**: Полный цикл запуска процессов.
- **Black Box**: Тестируем консоль как внешний процесс, не импортируя внутренние классы (максимальная реалистичность).

## 5. ROADMAP
- [ ] **Step 1**: Настройка окружения и фикстур (Server/Agent Lifecycle). `01_Environment_Setup.md`
- [ ] **Step 2**: Реализация сценариев тестирования (Happy Path & Errors). `02_Test_Scenarios.md`
- [ ] **Step 3**: Документация и запуск. `03_Docs_and_Run.md`

## 6. Global Checklist
todos:
  - id: setup-fixtures
    content: Создать conftest.py с фикстурами управления процессами (server, agent)
    status: pending
    time_estimate: 1h
    dependencies: []
  - id: impl-basic-tests
    content: Реализовать тесты list, status, info
    status: pending
    time_estimate: 1h
    dependencies: [setup-fixtures]
  - id: impl-interactive-tests
    content: Реализовать тесты управления агентами (sleep, wake, kill)
    status: pending
    time_estimate: 2h
    dependencies: [impl-basic-tests]
  - id: docs
    content: Обновить README.md инструкцией по запуску тестов
    status: pending
    time_estimate: 30m
    dependencies: [impl-interactive-tests]
