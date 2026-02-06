# Step 3: Verification & Docs

## A. Context Setup
- **Target Files**:
  - `plans/2026-01-09_FEATURE_SLEEP_PLAN/03_Admin_API_UI.md` (Update docs)
- **Reference Files**:
  - `Linux/MyCustomProjects/RevSocks_my/revsocks/tools/console/main.py`

## B. Detailed Design
После миграции нужно обновить документацию и провести финальную проверку.

## C. Implementation Steps

### 1. Manual Testing Protocol
1. **Startup**: Запуск без сервера (должен ругнуться красиво) и с сервером.
2. **Help**: Проверка автогенерации справки.
3. **Commands**:
   - `agents list`
   - `agent sleep target-id 30 -j 15`
   - `agent wake target-id`
   - `session kill target-id`

### 2. Doc Update
Обновить `plans/2026-01-09_FEATURE_SLEEP_PLAN/03_Admin_API_UI.md`, убрав упоминания Grumble.

## D. Verification
- [ ] Консоль запускается.
- [ ] Ошибки выводятся через `console.print` (Rich), а не traceback.
- [ ] Ctrl+C и Ctrl+D (EOF) работают корректно (cmd2 это умеет).

## E. Local Checklist
```yaml
todos:
  - id: manual-verification
    content: Провести ручное тестирование всех команд
    status: pending
  - id: docs-cleanup
    content: Обновить references в документации
    status: pending
```
