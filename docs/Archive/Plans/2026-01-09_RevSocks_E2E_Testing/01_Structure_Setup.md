# 01_Structure_Setup.md

## A. Context Setup
*   **Target Directory:** `tests/e2e/`
*   **Target File:** `tests/e2e/main_test.go`
*   **Reference Files:** `go.mod` (проверка зависимостей)

## B. Detailed Design
Создание базовой точки входа для тестов. Go позволяет использовать функцию `TestMain(m *testing.M)`, которая выполняется один раз перед всеми тестами. Это идеальное место для:
1.  Компиляции бинарников (Setup).
2.  Запуска тестов (`m.Run()`).
3.  Очистки временных файлов (Teardown).

### Directory Structure
```text
/tests
  /e2e
    ├── main_test.go      # Entry point (TestMain)
    ├── framework.go      # Common structs (TestContext)
    ├── builder.go        # Build logic (компиляция бинарника)
    ├── process.go        # Process control (Start/Stop/WaitForLog)
    ├── target.go         # Echo server для тестирования
    ├── traffic.go        # SOCKS5 клиент для проверки проксирования
    └── scenarios_test.go # Actual tests (TestE2E_Basic, TestE2E_Reconnect)
```

## C. Implementation Steps

### 1. Create Directory
```bash
mkdir -p tests/e2e
```

### 2. Create `framework.go` (Stub)
Определим основные структуры, чтобы `main_test.go` мог на них ссылаться.
```go
package e2e

import "testing"

// TestContext хранит глобальные настройки
type TestContext struct {
    ServerPath string
    ClientPath string
}

var GlobalCtx TestContext
```

### 3. Create `main_test.go`
```go
package e2e

import (
    "fmt"
    "os"
    "testing"
)

func TestMain(m *testing.M) {
    // 1. Setup: Build binaries
    if err := SetupEnvironment(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to setup environment: %v\n", err)
        os.Exit(1)
    }

    // 2. Run tests
    code := m.Run()

    // 3. Teardown
    TeardownEnvironment()

    os.Exit(code)
}

func SetupEnvironment() error {
    // TODO: Call Builder here
    return nil
}

func TeardownEnvironment() {
    // TODO: Cleanup temp dir
}
```

## D. Verification
*   **Manual:** Запуск `go test ./tests/e2e/...` должен проходить (пока без тестов, но компилироваться).

## E. Local Checklist
```yaml
todos:
  - id: create-framework-stub
    content: Создать framework.go с базовыми типами
    status: pending
  - id: create-main-test
    content: Создать main_test.go с TestMain
    status: pending
```

## F. Next Action
Prompt:
```text
Твоя задача — реализовать шаг 02: Binary Builder.
Нужно написать логику компиляции `cmd/revsocks-server` и `cmd/revsocks-client` во временную директорию.
Используй `os/exec` для вызова `go build`.
Обнови `SetupEnvironment` в `main_test.go`.
```
