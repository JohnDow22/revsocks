# Step 1: Environment Setup & Fixtures

## A. Context Setup (Critical)
- **Target Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/conftest.py` (New)
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/utils.py` (New)
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/main.py`
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/config.py`

## B. Detailed Design
Необходимо создать надежный механизм управления процессами для тестов.
1. **Dynamic Ports**: Сервер не должен занимать фиксированные порты (8081, 1080), чтобы тесты могли бежать параллельно или не конфликтовать с dev-окружением.
2. **Process Lifecycle**: Фикстуры должны гарантированно убивать процессы после тестов (`yield` pattern).
3. **Wait Strategy**: Тест должен ждать готовности сервера (health check) перед запуском консоли.

### Fixtures Plan
- `revsocks_bin`: Пути к бинарникам (из ENV или дефолтные).
- `free_port`: Генератор свободных портов.
- `revsocks_server`: Запускает сервер, возвращает (url, admin_token, process).
- `revsocks_agent`: Запускает агента, подключает к серверу.
- `console_env`: Подготавливает environment variables (`REVSOCKS_URL`, `REVSOCKS_TOKEN`) для запуска CLI.

## C. Implementation Steps

### 1. Подготовка структуры
Создать директорию `tests` внутри `tools/console`.

### 2. Реализация `conftest.py`
```python
import pytest
import subprocess
import time
import os
import socket
# ... imports ...

@pytest.fixture(scope="session")
def revsocks_binaries():
    # Поиск бинарников
    pass

@pytest.fixture
def revsocks_server(revsocks_binaries, free_port):
    # Запуск сервера
    # Ожидание healthcheck
    # yield config
    # kill process
    pass
```

## D. Verification
- **Automated**: Запустить простой тест, который просто проверяет, что фикстура поднимает сервер.
  ```bash
  pytest Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/tests/ --collect-only
  ```

## E. Local Checklist
todos:
  - id: create-dir
    content: Создать структуру папок тестов
    status: pending
    time_estimate: 5m
    dependencies: []
  - id: write-conftest
    content: Написать conftest.py с управлением процессами
    status: pending
    time_estimate: 45m
    dependencies: [create-dir]
  - id: verify-fixtures
    content: Проверить запуск и остановку процессов
    status: pending
    time_estimate: 10m
    dependencies: [write-conftest]

## F. Next Action
Промпт для реализации сценариев тестирования.
