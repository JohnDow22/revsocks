# Step 3: Documentation & Integration

## A. Context Setup (Critical)
- **Target Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/README.md`
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/requirements-dev.txt` (New)
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/requirements.txt`

## B. Detailed Design
Чтобы разработчики могли запустить тесты, нужно:
1. Описать зависимости (`pexpect`, `pytest`).
2. Описать процесс запуска (где взять бинарники, как настроить переменные окружения).

## C. Implementation Steps

### 1. Requirements
Создать `requirements-dev.txt`:
```text
pytest>=7.0.0
pexpect>=4.8.0
requests>=2.28.0
```

### 2. Documentation
Обновить `README.md` в секции "Testing":
- Команда установки зависимостей.
- Команда запуска тестов.
- Переменные окружения (`TEST_SERVER_BIN`, `TEST_AGENT_BIN`).

## D. Verification
- **Manual**: Прочитать README.md, выполнить инструкции в чистом окружении (venv).

## E. Local Checklist
todos:
  - id: create-reqs
    content: Создать requirements-dev.txt
    status: pending
    time_estimate: 5m
    dependencies: []
  - id: update-readme
    content: Обновить README.md
    status: pending
    time_estimate: 15m
    dependencies: [create-reqs]

## F. Next Action
Plan Complete.
