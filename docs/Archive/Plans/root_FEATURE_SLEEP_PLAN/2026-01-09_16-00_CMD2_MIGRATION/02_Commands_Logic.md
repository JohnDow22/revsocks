# Step 2: Commands Logic Refactor

## A. Context Setup
- **Target Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/commands/agents.py`
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/main.py`
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/core/api.py`

## B. Detailed Design
В `cmd2` для сложных аргументов используется `argparse`.
Вместо плоского списка команд `agents list`, `agent sleep`, мы используем **Subcommands**.

### 1. Command Structure
1. `agents` (Command)
   - `list` (Subcommand) -> показывает таблицу.
2. `agent` (Command)
   - `sleep <id> <interval> [--jitter]`
   - `wake <id>`
   - `rename <id> <alias>`
   - `delete <id>`
3. `session` (Command)
   - `kill <id>`

### 2. Refactoring `commands/agents.py`
Превращаем этот файл в модуль, содержащий **Mixin** класс `AgentCommandsMixin`.
`main.py` будет наследовать `RevSocksConsole` от `cmd2.Cmd` и `AgentCommandsMixin`.

```python
# commands/agents.py
import argparse
from cmd2 import with_argparser, with_category

class AgentCommandsMixin:
    # ... logic ...
```

### 3. Argument Parsers
Для каждой группы команд создаем отдельный парсер с подпарсерами.

**Пример для `agent`:**
```python
agent_parser = argparse.ArgumentParser()
agent_subparsers = agent_parser.add_subparsers(dest='subcommand', required=True)

# sleep
sleep_parser = agent_subparsers.add_parser('sleep')
sleep_parser.add_argument('agent_id')
sleep_parser.add_argument('interval', type=int)

# wake
wake_parser = agent_subparsers.add_parser('wake')
# ...
```

## C. Implementation Steps

### 1. Refactor `agents.py`
- Удалить зависимость от `grumble`.
- Создать класс `AgentCommandsMixin`.
- Определить `argparse` парсеры как атрибуты класса или глобально.
- Реализовать методы `do_agents`, `do_agent`, `do_session` с декоратором `@with_argparser`.
- Внутри методов использовать `args.subcommand` для диспетчеризации.

### 2. Integrate into `main.py`
- Импортировать `AgentCommandsMixin`.
- Добавить его в наследование: `class RevSocksConsole(cmd2.Cmd, AgentCommandsMixin): ...`
- В `__init__` передать `api_client` (так как миксин будет ожидать `self.api_client`).

## D. Verification
### Manual
- `python main.py`
- `agent sleep 123 60` -> должно вызвать API.
- `agent --help` -> должно показать подкоманды.
- `agents list` (или просто `agents`) -> таблица.

## E. Local Checklist
```yaml
todos:
  - id: mixin-create
    content: Создать AgentCommandsMixin в commands/agents.py
    status: pending
  - id: parsers-setup
    content: Настроить argparse для agent/agents/session
    status: pending
  - id: main-integrate
    content: Подключить Mixin в main.py
    status: pending
```
