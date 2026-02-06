# 03_Process_Manager.md

## A. Context Setup
*   **Target File:** `tests/e2e/process.go`
*   **Reference Files:** `tests/e2e/framework.go`

## B. Detailed Design
Нам нужен надежный способ запускать сервер и клиент, ждать пока они "прогреются" (откроют порты) и убивать их после теста.

### Structs
```go
type Process struct {
    Name    string
    Cmd     *exec.Cmd
    Stdout  *bytes.Buffer
    Stderr  *bytes.Buffer
}
```

### Methods
*   `Start(args ...string) error`
*   `Stop() error`
*   `WaitForLog(pattern string, timeout time.Duration) error` — критически важно для синхронизации (ждать "Listening on...").

## C. Implementation Steps

### 1. Create `tests/e2e/process.go`
```go
package e2e

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "os/exec"
    "strings"
    "time"
)

type Process struct {
    BinPath string
    Name    string
    cmd     *exec.Cmd
    stdout  bytes.Buffer
    stderr  bytes.Buffer
    // Каналы для стриминга логов, чтобы WaitForLog работал в реальном времени
}

func NewProcess(binPath, name string) *Process {
    return &Process{BinPath: binPath, Name: name}
}

func (p *Process) Start(args ...string) error {
    p.cmd = exec.Command(p.BinPath, args...)
    // Настройка Pipes...
    return p.cmd.Start()
}

func (p *Process) Stop() error {
    if p.cmd == nil || p.cmd.Process == nil {
        return nil
    }
    // Сначала Try Graceful (SIGTERM), потом Kill
    return p.cmd.Process.Kill()
}

// WaitForLog сканирует output и ждет подстроку.
func (p *Process) WaitForLog(pattern string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        output := p.stdout.String() + p.stderr.String()
        if strings.Contains(output, pattern) {
            return nil
        }
        time.Sleep(100 * time.Millisecond)
    }
    return fmt.Errorf("timeout waiting for log pattern: %q", pattern)
}

// GetOutput возвращает объединённый вывод для отладки
func (p *Process) GetOutput() string {
    return fmt.Sprintf("=== STDOUT ===\n%s\n=== STDERR ===\n%s", 
        p.stdout.String(), p.stderr.String())
}
```

## D. Verification
*   **Manual:** Написать unit-test для `process.go`, который запускает `sleep` или `echo`.

## E. Local Checklist
```yaml
todos:
  - id: impl-process-struct
    content: Реализовать структуру Process
    status: pending
  - id: impl-wait-log
    content: Реализовать логику ожидания лога (WaitForLog)
    status: pending
```

## F. Next Action
Prompt:
```text
Твоя задача — реализовать шаг 04: Traffic Generator & Target.
Нужно создать:
1. `target.go`: HTTP/TCP эхо-сервер, который слушает на порту :0 (рандомный) и возвращает то, что получил.
2. `traffic.go`: Утилиту, которая использует SOCKS5 прокси (через `golang.org/x/net/proxy`) для подключения к target.
```
