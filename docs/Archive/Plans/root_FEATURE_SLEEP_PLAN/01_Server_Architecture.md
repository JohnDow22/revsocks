# Step 1: Server Core Architecture (AgentManager & Handshake)

## ⚠️ ОБНОВЛЕНО: Учтён рефакторинг (2026-01-09)
Код перенесён в `internal/server/`, `cmd/server/`. Используем новую структуру проекта.

## A. Context Setup

### Target Files
- `internal/server/agent_manager.go` (New)
- `internal/server/server.go` (Modification)
- `cmd/server/main.go` (Modification - инициализация AgentManager)

### Reference Files
- `internal/server/session.go` (SessionManager для понимания паттернов)
- `internal/common/protocol.go` (для версий протокола)
- `feature.md` (описание новой фичи)

## B. Detailed Design

### 1. Agent Metadata Structure
```go
type AgentState string

const (
    StateTunnel AgentState = "TUNNEL"
    StateSleep  AgentState = "SLEEP"
)

type AgentConfig struct {
    ID            string     `json:"id"`
    Alias         string     `json:"alias"`
    Mode          AgentState `json:"mode"`
    SleepInterval int        `json:"sleep_interval"` // Seconds
    Jitter        int        `json:"jitter"`         // Percent (0-100)
    LastSeen      time.Time  `json:"last_seen"`
    FirstSeen     time.Time  `json:"first_seen"`
    IP            string     `json:"ip"`
}
```

### 2. AgentManager Interface
```go
type AgentManager struct {
    agents map[string]*AgentConfig
    mu     sync.RWMutex
    dbPath string
}

func NewAgentManager(path string) (*AgentManager, error)
func (m *AgentManager) Load() error
func (m *AgentManager) Save() error
func (m *AgentManager) GetConfig(id string) *AgentConfig
func (m *AgentManager) RegisterAgent(id, ip string) (*AgentConfig, error)
func (m *AgentManager) UpdateState(id string, mode AgentState, interval, jitter int) error
```

### 3. Handshake Protocol v3
**Format:** Text-based, newline delimited.
1.  **Client:** `AUTH <password> <agent_id> <version>\n`
2.  **Server:**
    *   If Auth Fail: `ERR Auth Failed\n` (Close)
    *   If Tunnel Mode: `CMD TUNNEL\n` (Proceed to Yamux)
    *   If Sleep Mode: `CMD SLEEP <interval> <jitter>\n` (Close)

## C. Implementation Steps

1.  **Create `internal/server/agent_manager.go`:**
    - Implement `AgentConfig` struct.
    - Implement `Load/Save` methods (JSON marshalling).
    - Implement thread-safe `RegisterAgent` (create default if not exists).
    - Implement `GetConfig`, `UpdateConfig`, `ListAgents` methods.

2.  **Modify `internal/server/server.go`:**
    - Add `AgentManager` field to `agentHandler` struct.
    - Update `handleConnection` function:
        - Read line scanning for `AUTH <password> <agent_id> <version>`.
        - Validate password.
        - Call `manager.RegisterAgent(id, remoteAddr)`.
        - Check `agent.Mode`.
        - Send appropriate response (`CMD TUNNEL` | `CMD SLEEP <sec> <jitter>` | `ERR <msg>`).
        - If `TUNNEL`: continue to `yamux.Server`.
        - If `SLEEP`: return (close connection).

3.  **Modify `cmd/server/main.go`:**
    - Initialize `AgentManager` before starting server.
    - Pass `AgentManager` to server config/handler.
    - Add config option for `agents.json` path (default: `./agents.json`).

## D. Verification

### Manual
1.  Use `netcat` to simulate client:
    ```bash
    echo "AUTH mypassword agent1 v1" | nc localhost 8080
    ```
2.  Check response: `CMD TUNNEL` (default).
3.  Check `agents.json` created.

### Automated
- **Unit Test:** `TestAgentManager_SaveLoad` in `agent_manager_test.go`.
- **Unit Test:** `TestHandshakeParser` in `rserver_test.go`.

## E. Local Checklist
```yaml
todos:
  - id: create-agent-manager
    content: Создать internal/server/agent_manager.go с struct и JSON logic
    status: pending
  - id: mod-handshake
    content: Рефакторить handleConnection в internal/server/server.go для v3 protocol
    status: pending
  - id: init-manager
    content: Инициализировать AgentManager в cmd/server/main.go
    status: pending
  - id: unit-tests-srv
    content: Написать тесты для persistence и handshake (internal/server/)
    status: pending
```

## F. Next Action
Proceed to Client Implementation ([02_Client_Architecture.md]) to implement the other side of the protocol.
