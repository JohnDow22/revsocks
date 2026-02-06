# Step 2: Client Core Logic (Beacon Loop & Jitter)

## ⚠️ ОБНОВЛЕНО: Учтён рефакторинг (2026-01-09)
Код перенесён в `internal/agent/`, `cmd/agent/`. Используем новую структуру проекта.

## A. Context Setup

### Target Files
- `internal/agent/client.go` (Modification)
- `cmd/agent/main.go` (Modification - инициализация beacon loop)

### Reference Files
- `internal/common/rand.go` (для генерации случайных значений)
- `internal/common/version.go` (для версии протокола)
- `feature.md` (описание фичи)

## B. Detailed Design

### 1. Beacon Loop Logic
Instead of a simple `connect -> run` loop, the client becomes a state machine.

```go
func startBeaconLoop(config *Config) {
    currentInterval := config.InitialInterval
    
    for {
        conn, cmd, params, err := connectAndHandshake(config)
        if err != nil {
            // Log error, sleep default backoff, continue
            time.Sleep(10 * time.Second)
            continue
        }

        switch cmd {
        case "TUNNEL":
            // Start Yamux and block until finished
            runTunnel(conn)
            // If tunnel breaks, loop immediately (or with small delay)
            
        case "SLEEP":
            // Parse params: interval, jitter
            // conn is already closed by server or we close it
            conn.Close()
            sleepDuration := calculateJitter(params.Interval, params.Jitter)
            time.Sleep(sleepDuration)
            
        default:
            // Unknown command, sleep default
            conn.Close()
            time.Sleep(1 * time.Minute)
        }
    }
}
```

### 2. Jitter Calculation
```go
func calculateJitter(baseSeconds int, jitterPercent int) time.Duration {
    // jitterPercent = 10 means +/- 10%
    // Range: [base * 0.9, base * 1.1]
    
    delta := float64(baseSeconds) * (float64(jitterPercent) / 100.0)
    min := float64(baseSeconds) - delta
    max := float64(baseSeconds) + delta
    
    randomSec := min + rand.Float64() * (max - min)
    return time.Duration(randomSec * float64(time.Second))
}
```

### 3. ID Generation
The client needs a persistent ID.
- On first run: Generate UUID.
- Save to `~/.revsocks.id` (or relative path).
- On next run: Read from file.

## C. Implementation Steps

1.  **Refactor `internal/agent/client.go`:**
    - Add `loadOrGenerateAgentID()` function (save to `~/.revsocks.id` or relative path).
    - Add `Config.AgentID` field.
    - Rename/update `connect()` to `connectAndHandshake()`.
    - Implement AUTH protocol (send `AUTH <pass> <id> <version>`).
    - Parse Server response (`CMD TUNNEL` | `CMD SLEEP <sec> <jitter>` | `ERR <msg>`).
    - Implement `startBeaconLoop` - заменить текущий reconnect loop.
    - Implement `calculateJitter` function.
    - Export `StartBeaconLoop()` для использования в cmd/agent.

2.  **Update `cmd/agent/main.go`:**
    - Load/generate Agent ID при старте.
    - Передать ID в agent.Config.
    - Вызвать `agent.StartBeaconLoop(config)` вместо прямого connect.

## D. Verification

### Manual
1.  Run modified Server.
2.  Run modified Client.
3.  Observe Client output: "Received SLEEP 60 10".
4.  Wait ~60s.
5.  Observe Reconnect.
6.  Change Agent Mode to TUNNEL on server (manually editing json).
7.  Observe Client entering Tunnel mode.

### Automated
- **Unit Test:** `TestJitter` (ensure distribution is correct).
- **Unit Test:** `TestIDGeneration` (ensure persistence).

## E. Local Checklist
```yaml
todos:
  - id: client-id
    content: Реализовать persistent Agent ID generation в internal/agent/client.go
    status: pending
  - id: client-loop
    content: Реализовать Beacon Loop и Handshake parsing в internal/agent/client.go
    status: pending
  - id: client-jitter
    content: Реализовать Jitter logic в internal/agent/client.go
    status: pending
  - id: client-main-init
    content: Обновить cmd/agent/main.go для beacon loop
    status: pending
  - id: unit-tests-cli
    content: Написать тесты для Jitter и ID generation (internal/agent/)
    status: pending
```

## F. Next Action
Proceed to Admin API Implementation ([03_Admin_API_UI.md]).
