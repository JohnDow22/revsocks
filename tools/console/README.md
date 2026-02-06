# RevSocks Admin Console

Python CLI для управления RevSocks агентами через Admin API.

## Установка

```bash
cd tools/console
pip install -r requirements.txt
chmod +x main.py
```

## Настройка

Укажите токен и URL сервера через переменные окружения:

```bash
export REVSOCKS_TOKEN="your-admin-token-from-server"
export REVSOCKS_URL="http://127.0.0.1:8081"  # опционально, default: 127.0.0.1:8081
```

## Запуск

```bash
./main.py
# или
python3 main.py
```

## Команды

### Управление агентами

- `agents list [-v]` - список всех агентов (с флагом `-v` показывает подробности)
- `agent sleep <agent_id> <interval> [-j jitter]` - перевести агента в SLEEP режим
- `agent wake <agent_id>` - перевести агента в TUNNEL режим (постоянное соединение)
- `agent rename <agent_id> <alias>` - установить человекочитаемый алиас
- `agent delete <agent_id> [-f]` - удалить агента из базы

### Управление сессиями

- `session kill <agent_id>` - убить активную сессию (закрыть yamux)

### Системные команды

- `status` - проверка соединения с сервером
- `info` - информация о консоли
- `help` - список команд
- `exit` - выход

## Примеры использования

```bash
revsocks> agents list
revsocks> agent sleep server1 60 -j 10
revsocks> agent wake server1
revsocks> agent rename server1 "Production Web Server"
revsocks> session kill server1
```

## Безопасность

- Токен передаётся через `X-Admin-Token` header
- Не храните токен в истории команд
- Используйте переменные окружения для конфигурации

## Тестирование

### Установка зависимостей для разработки

```bash
cd tools/console
pip install -r requirements-dev.txt
```

### Запуск E2E тестов

Тесты автоматически запускают сервер и агента на динамических портах.

**Требования:**
- Собранные бинарники `revsocks-server` и `revsocks-agent-test` в директории `revsocks/`
- `revsocks-agent-test` - обычная сборка агента (без baked конфига), собрать: `go build -o revsocks-agent-test ./cmd/agent/`
- Или указать пути через переменные окружения

```bash
# Запуск всех тестов
pytest tests/ -v

# Только базовые тесты (без агента)
pytest tests/test_e2e_basic.py -v

# Тесты с агентом
pytest tests/test_e2e_interactive.py -v
```

### Переменные окружения для тестов

| Переменная | Описание | По умолчанию |
|---|---|---|
| `TEST_SERVER_BIN` | Путь к бинарнику сервера | `../../revsocks-server` |
| `TEST_AGENT_BIN` | Путь к бинарнику агента | `../../revsocks-agent-test` |

Пример с кастомными путями:

```bash
export TEST_SERVER_BIN=/path/to/revsocks-server
export TEST_AGENT_BIN=/path/to/revsocks-agent
pytest tests/ -v
```

### Структура тестов

- `tests/conftest.py` - фикстуры для управления процессами (server, agent)
- `tests/test_e2e_basic.py` - базовые тесты (status, info, help, ошибки авторизации)
- `tests/test_e2e_interactive.py` - тесты с агентом (agents list, sleep, wake, rename)
