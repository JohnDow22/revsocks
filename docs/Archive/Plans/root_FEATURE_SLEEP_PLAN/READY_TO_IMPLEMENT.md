# ✅ ПЛАН ОБНОВЛЁН - ГОТОВ К РЕАЛИЗАЦИИ

## Статус: READY TO IMPLEMENT

Дата обновления: **2026-01-09**

---

## 📋 Что было сделано

### 1. Анализ выполненного рефакторинга
✅ Изучена новая структура проекта после `2026-01-09_RevSocks_Refactor`:
- `cmd/agent/` и `cmd/server/` - entry points
- `internal/{agent,server,common,transport}/` - изолированная логика
- Размер агента уменьшен: ~10.8 MB (vs 13.4 MB сервер)

### 2. Обновление всех документов плана
✅ **00_PLAN_INDEX.md**
- Добавлено предупреждение о рефакторинге
- Обновлена матрица зависимостей
- Пересмотрен Global Checklist (10 задач с правильными путями)

✅ **01_Server_Architecture.md**
- Target Files: `internal/server/agent_manager.go`, `internal/server/server.go`, `cmd/server/main.go`
- Разделены задачи: создание в internal/, инициализация в cmd/

✅ **02_Client_Architecture.md**
- Target Files: `internal/agent/client.go`, `cmd/agent/main.go`
- Добавлен экспорт функций для использования в cmd/

✅ **03_Admin_API_UI.md**
- Target Files: `internal/server/api.go`, `cmd/server/main.go`, `tools/console/`
- Добавлен шаг инициализации Admin API

✅ **04_Testing_Strategy.md**
- Обновлены пути к тестам в `internal/*/`
- Интеграция с существующим `tests/e2e/` framework
- Расширены сценарии тестирования

✅ **05_Next_Steps.md** (полностью переписан)
- Разбит на 5 фаз с детальным breakdown
- Добавлена матрица зависимостей (Mermaid diagram)
- Раздел Risk Mitigation
- Оценка времени: **12-17 часов**

### 3. Дополнительная документация
✅ Создан `PLAN_UPDATE_LOG.md` с подробным описанием изменений
✅ Создан `READY_TO_IMPLEMENT.md` (этот файл)

---

## 🎯 Что реализуем

### Фича: Beacon Mode (Sleep/Check-in режим)

**Цель:** Агенты RevSocks смогут уходить в сон на N секунд, периодически подключаясь к серверу для проверки команд (check-in), вместо постоянного TCP-соединения.

**Преимущества:**
- 🔒 **Stealth**: меньше сетевых соединений → сложнее детектировать
- 🛡️ **Обход детекций**: нет постоянных long-lived connections
- ⚙️ **Гибкость**: админ может динамически переключать агентов между SLEEP и TUNNEL режимами

---

## 📦 Архитектурные решения

### Протокол: Handshake v3 (текстовый)
```
Client → Server: AUTH <password> <agent_id> <version>
Server → Client: CMD TUNNEL | CMD SLEEP <sec> <jitter> | ERR <message>
```

### Persistence: JSON файл
- **Файл:** `agents.json` (по умолчанию в рабочей директории сервера)
- **Формат:** Array of AgentConfig
- **Thread-safety:** In-Memory Map + RWMutex

### State Machine (Agent Side)
```
Loop:
  1. Connect to Server
  2. Send AUTH
  3. Receive Command
  4. If TUNNEL → Start Yamux → Block until disconnect
  5. If SLEEP → Close connection → Sleep(interval + jitter) → Repeat
```

---

## 🛠️ Roadmap реализации

### Phase 1: Server Core (2-3 часа)
**Файлы:**
- ✨ NEW: `internal/server/agent_manager.go`
- 🔧 MODIFY: `internal/server/server.go`
- 🔧 MODIFY: `cmd/server/main.go`

**Задачи:**
1. AgentManager struct + JSON persistence
2. Handshake v3 в handleConnection
3. Инициализация в main.go

**Verification:** Unit tests + компиляция

---

### Phase 2: Client Core (3-4 часа)
**Файлы:**
- 🔧 MODIFY: `internal/agent/client.go`
- 🔧 MODIFY: `cmd/agent/main.go`

**Задачи:**
1. Persistent Agent ID (`~/.revsocks.id`)
2. Beacon Loop (SLEEP/TUNNEL state machine)
3. Jitter calculation (random sleep)

**Verification:** Unit tests + manual test (agent ↔ server)

---

### Phase 3: Admin API (2-3 часа)
**Файлы:**
- ✨ NEW: `internal/server/api.go`
- 🔧 MODIFY: `cmd/server/main.go`

**Endpoints:**
- `GET /api/agents` - список агентов
- `POST /api/agents/{id}/config` - изменить режим
- `DELETE /api/sessions/{id}` - убить активную сессию

**Verification:** curl тесты

---

### Phase 4: Console UI (3-4 часа)
**Файлы:**
- ✨ NEW: `tools/console/` (Python project)

**Структура:**
```
tools/console/
├── pyproject.toml (Poetry)
├── main.py (Grumble entrypoint)
├── core/api.py (HTTP wrapper)
└── commands/agents.py (CLI commands)
```

**Commands:**
- `agents list` - table view
- `agent sleep <id> <seconds>` - set sleep mode
- `agent wake <id>` - set tunnel mode
- `agent rename <id> <alias>` - set alias

**Verification:** Manual testing

---

### Phase 5: Testing & Docs (2-3 часа)
**Файлы:**
- ✨ NEW: `internal/server/agent_manager_test.go`
- ✨ NEW: `internal/agent/client_test.go`
- ✨ NEW: `internal/server/api_test.go`
- 🔧 MODIFY: `tests/e2e/scenarios_test.go`
- ✨ NEW: `docs/04_Features/BEACON_MODE.md`

**Verification:** `go test ./... -v` - все тесты проходят

---

## ⏱️ Временная оценка

| Phase | Задачи | Время |
|---|---|---|
| **Phase 1** | Server Core | 2-3 часа |
| **Phase 2** | Client Core | 3-4 часа |
| **Phase 3** | Admin API | 2-3 часа |
| **Phase 4** | Console UI | 3-4 часа |
| **Phase 5** | Testing & Docs | 2-3 часа |
| **TOTAL** | - | **12-17 часов** |

---

## 🚀 Следующий шаг

**START HERE:**

```bash
cd /home/dark/BTC/ZK/2018/MyProjects/Sonnet_4+Memory_bank/Hack/Pentest/Linux/MyCustomProjects/RevSocks_my/revsocks

# Phase 1, Task 1
touch internal/server/agent_manager.go
```

**Документация:** См. `plans/2026-01-09_FEATURE_SLEEP_PLAN/01_Server_Architecture.md`

---

## ✅ Pre-Flight Checklist

- [x] Рефакторинг завершён и протестирован
- [x] Все документы плана обновлены
- [x] Пути к файлам корректны
- [x] TODO-списки синхронизированы
- [x] Ссылки между документами проверены
- [x] Временные оценки пересчитаны
- [x] Risk mitigation стратегия определена
- [x] E2E тесты готовы к расширению

---

## 📚 Справочные материалы

### Документы плана
1. `00_PLAN_INDEX.md` - обзор, ADL, roadmap
2. `01_Server_Architecture.md` - детальный дизайн сервера
3. `02_Client_Architecture.md` - детальный дизайн клиента
4. `03_Admin_API_UI.md` - API и Console UI
5. `04_Testing_Strategy.md` - стратегия тестирования
6. `05_Next_Steps.md` - implementation plan (этот план)

### Существующая кодовая база
- `internal/server/session.go` - паттерны управления сессиями
- `internal/common/protocol.go` - версии протокола
- `tests/e2e/` - E2E testing framework
- `docs/04_Features/SESSION_LIFECYCLE_MANAGER.md` - lifecycle management

### Правила разработки
- `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc`
- `.cursor/rules/Dev_2.0/quality/UI/Grumble/Grumble_UI.mdc`

---

## 🎉 План готов к реализации!

**Все изменения учтены, все пути обновлены, все зависимости проверены.**

Можно начинать Phase 1! 🚀
