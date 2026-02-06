# Step 3: Admin API & Console UI

## ⚠️ ОБНОВЛЕНО: Учтён рефакторинг (2026-01-09)
Код перенесён в `internal/server/`, `cmd/server/`. Используем новую структуру проекта.

## A. Context Setup

### Target Files
- `internal/server/api.go` (New)
- `cmd/server/main.go` (Modification - запуск Admin API)
- `tools/console/` (New Directory)

### Reference Files
- `.cursor/rules/Dev_2.0/quality/UI/Grumble/Grumble_UI.mdc`
- `internal/server/agent_manager.go` (для интеграции с API)

## B. Detailed Design

### 1. Server API (Go)
Use standard `net/http` to minimize dependencies.
**Auth:** Header `X-Admin-Token` (configured in YAML).

| Method | Path | Body | Description |
| :--- | :--- | :--- | :--- |
| GET | `/api/agents` | - | List all known agents |
| POST | `/api/agents/{id}/config` | `{"mode": "SLEEP", "interval": 60}` | Update agent config |
| DELETE | `/api/sessions/{id}` | - | Kill active tunnel (close yamux) |

### 2. Console UI (Python + Grumble)
Follow `Grumble_UI.mdc` structure.

**Project Structure:**
```
tools/console/
├── main.py
├── config.yaml
├── core/
│   ├── api.py (Requests wrapper)
├── commands/
│   ├── agents.py (list, set-sleep, set-tunnel)
│   ├── sessions.py (kill)
```

**Commands:**
- `agents list` -> Table [ID, Alias, Mode, IP, LastSeen]
- `agent sleep <id> <seconds>` -> API POST
- `agent wake <id>` -> API POST (Mode=TUNNEL)
- `agent rename <id> <alias>` -> API POST

## C. Implementation Steps

### Server Side
1.  **Create `internal/server/api.go`:**
    - Implement `AdminServer` struct с полями: `manager *AgentManager`, `token string`.
    - Implement `StartAdminServer(port, token, manager)` - запуск HTTP сервера.
    - Handlers для `GET /api/agents` (read from manager).
    - Handlers для `POST /api/agents/{id}/config` (update manager + save).
    - Handlers для `DELETE /api/sessions/{id}` (kill active yamux session).
    - Auth middleware (проверка `X-Admin-Token` header).

2.  **Modify `cmd/server/main.go`:**
    - Add flags: `-admin-api`, `-admin-port`, `-admin-token`.
    - Если `-admin-api` включён - запустить `AdminServer` в отдельной горутине.
    - Передать `AgentManager` в AdminServer.

### Client Side (Console)
1.  **Setup Project:** `pip install grumble requests rich`.
2.  **Implement `core/api.py`:** Wrapper для HTTP calls с токеном из config.
3.  **Implement `commands/agents.py`:** Логика для `list`, `sleep`, `wake`, `rename`.
4.  **Implement `main.py`:** App entrypoint с Grumble shell.

## D. Verification

### Manual
1.  Start Server.
2.  Start Console: `python3 tools/console/main.py`.
3.  Run `agents list` -> Empty table.
4.  Connect a Client.
5.  Run `agents list` -> See client.
6.  Run `agent sleep <id> 30`.
7.  Check Server logs -> Agent should receive SLEEP cmd on next check-in.

### Automated
- **API Tests:** Use `curl` or Postman collection to verify API endpoints.

## E. Local Checklist
```yaml
todos:
  - id: srv-api
    content: Реализовать HTTP API в internal/server/api.go
    status: pending
  - id: srv-api-init
    content: Запустить Admin API в cmd/server/main.go
    status: pending
  - id: py-console
    content: Реализовать Grumble CLI tool в tools/console/
    status: pending
  - id: api-tests
    content: Написать тесты для API endpoints
    status: pending
```

## F. Next Action
Proceed to Testing Strategy ([04_Testing_Strategy.md]).
