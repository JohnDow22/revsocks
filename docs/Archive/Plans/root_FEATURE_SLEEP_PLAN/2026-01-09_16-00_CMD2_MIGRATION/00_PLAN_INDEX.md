# Plan Index: RevSocks Console Migration (Grumble -> cmd2)

## 1. Goal
Мигрировать консоль администратора (`tools/console`) с несуществующей библиотеки `python-grumble` на стандартную библиотеку `cmd2`. Обеспечить функциональность REPL с сохранением текущей архитектуры команд и интеграции с API.

## 2. Decision Log
- **Проблема**: `pip install` падает, так как `python-grumble` нет в PyPI.
- **Решение**: Использовать `cmd2`.
- **Обоснование**: 
  - `cmd2` поддерживает `argparse` декораторы (минимальный рефакторинг `commands/agents.py`).
  - Встроенная поддержка истории, алиасов и шелл-команд.
  - Активная поддержка и документация.
- **Архитектура**:
  - `main.py`: Наследуемся от `cmd2.Cmd`.
  - `commands/`: Переход от `@arg` к `@with_argparser`.
  - Иерархия: `do_agent` + `argparse.subparsers` для имитации команд `agent sleep`, `agent wake`.

## 3. Матрица Зависимостей
- **Modules**:
  - `tools/console/requirements.txt`
  - `tools/console/main.py`
  - `tools/console/commands/agents.py`
- **External**:
  - `cmd2` (New)
  - `requests` (Existing)
  - `rich` (Existing)

## 4. Стратегия Тестирования
- **Unit Tests**: `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/templates/Testing/Testing_Level_1.mdc` (Basic Unit).
  - Тестирование парсинга аргументов.
  - Мокирование `APIClient`.
- **Manual Verification**:
  - Запуск оболочки.
  - Проверка help.
  - Выполнение команд `agents list`, `agent sleep ...`.

## 5. ROADMAP
1. **Infrastructure**: Обновление зависимостей и каркаса приложения (`main.py`). [🔴 Pending]
2. **Logic Refactor**: Адаптация команд агентов под `argparse`. [🔴 Pending]
3. **Verification**: Ручное и автоматическое тестирование. [🔴 Pending]

## 6. Global Checklist
```yaml
todos:
  - id: deps-update
    content: Заменить python-grumble на cmd2 в requirements.txt
    status: pending
    time_estimate: 5 мин
    dependencies: []
  
  - id: main-refactor
    content: Переписать main.py на cmd2.Cmd
    status: pending
    time_estimate: 20 мин
    dependencies: [deps-update]
  
  - id: commands-refactor
    content: Рефакторинг commands/agents.py (decorators -> argparse)
    status: pending
    time_estimate: 30 мин
    dependencies: [main-refactor]
    
  - id: manual-test
    content: Ручная проверка запуска и выполнения команд
    status: pending
    time_estimate: 10 мин
    dependencies: [commands-refactor]
```
