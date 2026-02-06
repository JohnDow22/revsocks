# Step 2: Test Scenarios Implementation

## A. Context Setup (Critical)
- **Target Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/test_e2e_basic.py` (New)
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/test_e2e_interactive.py` (New)
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/conftest.py`
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/commands/agents.py`

## B. Detailed Design
Используем `pexpect` для запуска `main.py` и взаимодействия с ним.

### Scenarios

#### 1. Basic Commands (`test_e2e_basic.py`)
- **`status`**: Проверка вывода "Server: healthy".
- **`info`**: Проверка статического вывода.
- **`help`**: Проверка наличия команд.
- **Auth Fail**: Запуск с неверным токеном -> ожидание ошибки и exit code 1.

#### 2. Interactive Agent Management (`test_e2e_interactive.py`)
- **Setup**: Поднять сервер + 1 агент.
- **`agents list`**:
  - Ввод: `agents list`
  - Ожидание: ID агента, IP, статус.
- **`agent sleep`**:
  - Ввод: `agent sleep <id> 60`
  - Ожидание: Success message.
  - Проверка (API): Проверить через `requests` к API сервера, что статус агента изменился.
- **`agent wake`**:
  - Ввод: `agent wake <id>`
  - Ожидание: Success message.
- **`agent rename`**:
  - Ввод: `agent rename <id> new_name`
  - Ожидание: Success message.
  - Проверка: `agents list` показывает новый алиас.

## C. Implementation Steps

### 1. Basic Tests
Создать `test_e2e_basic.py`.
Тест запускает `python3 main.py`, ожидает промпт `revsocks>`, отправляет команду, проверяет вывод.

### 2. Interactive Tests
Создать `test_e2e_interactive.py`.
Использовать фикстуру `revsocks_agent` для создания "живой" мишени.

## D. Verification
- **Automated**:
  ```bash
  pytest -v Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/
  ```

## E. Local Checklist
todos:
  - id: impl-basic
    content: Реализовать test_e2e_basic.py
    status: pending
    time_estimate: 30m
    dependencies: []
  - id: impl-interactive
    content: Реализовать test_e2e_interactive.py
    status: pending
    time_estimate: 1h
    dependencies: [impl-basic]
  - id: fix-bugs
    content: Исправить возможные проблемы с таймаутами pexpect
    status: pending
    time_estimate: 30m
    dependencies: [impl-interactive]

## F. Next Action
Документация и финальная проверка.
