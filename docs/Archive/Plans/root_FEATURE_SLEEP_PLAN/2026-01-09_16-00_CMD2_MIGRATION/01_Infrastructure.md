# Step 1: Infrastructure & Main Entrypoint

## A. Context Setup
- **Target Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/requirements.txt`
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/main.py`
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/config.py`

## B. Detailed Design
### 1. Dependencies
Заменить несуществующий пакет на `cmd2`.

### 2. Main Application Class (`RevSocksConsole`)
Вместо функциональной сборки `App` из `grumble`, создаем класс-наследник `cmd2.Cmd`.

```python
class RevSocksConsole(cmd2.Cmd):
    def __init__(self, config, api_client):
        super().__init__()
        self.config = config
        self.api_client = api_client
        self.prompt = config["console"]["prompt"] + " "
        self.intro = "RevSocks Admin Console..."
        # Регистрация команд происходит через миксины или прямой импорт,
        # но для простоты мы можем инстанцировать AgentCommands и 
        # прокинуть методы, либо (лучше) наследовать AgentCommands от cmd2.Cmd 
        # и использовать множественное наследование или композицию.
        
        # Решение: Композиция. Мы регистрируем методы do_* динамически или 
        # явно прописываем их в классе, делегируя выполнение объекту AgentCommands.
```

**Architecture Decision**: Для быстрой миграции и чистоты кода, методы `do_*` будут определены в `main.py` (или импортированы как миксины), и они будут вызывать логику из `AgentCommands`, которая вернет данные, а UI (rich) отрисует их.
Однако `cmd2` требует, чтобы методы `do_` были частью класса `Cmd`.
**Revised Design**: `AgentCommands` перестает быть просто классом с методами. Мы сделаем `RevSocksConsole` основным классом, а логику команд перенесем прямо в него (или в Mixin класс в `commands/agents.py`), чтобы декораторы `cmd2` работали корректно.

## C. Implementation Steps

### 1. Update Requirements
- Файл: `tools/console/requirements.txt`
- Действие: Замена строк.

### 2. Update Main Entrypoint
- Файл: `tools/console/main.py`
- Действие:
  1. Импорт `cmd2`.
  2. Удаление `grumble` кода.
  3. Создание класса `RevSocksConsole(cmd2.Cmd)`.
  4. Настройка `logging` и `APIClient` остается.
  5. Перенос логики `do_status`, `do_info` внутрь класса.
  6. **Важно**: Логика `agents` будет подключена на следующем этапе (через импорт Mixin или регистрацию). Пока оставляем заглушки.

## D. Verification
### Manual
- Запуск `pip install -r requirements.txt` должен пройти успешно.
- Запуск `python main.py` должен открыть шелл `cmd2`.
- Команда `help` должна работать.

## E. Local Checklist
```yaml
todos:
  - id: req-update
    content: Обновить requirements.txt
    status: pending
  - id: main-skeleton
    content: Создать каркас RevSocksConsole в main.py
    status: pending
```

## F. Next Action
Переход к рефакторингу команд агентов (`02_Commands_Logic.md`).
