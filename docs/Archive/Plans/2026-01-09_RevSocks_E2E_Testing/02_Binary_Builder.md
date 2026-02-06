# 02_Binary_Builder.md

## A. Context Setup
*   **Target File:** `tests/e2e/builder.go`
*   **Target File:** `tests/e2e/main_test.go` (update)
*   **Reference Files:** `main.go` (root), `go.mod`

## B. Detailed Design
Модуль `builder` отвечает за сборку актуального бинарника перед запуском тестов.
**Важно:** `RevSocks` является монолитным приложением. Режим работы (сервер/клиент) определяется флагами `-listen` и `-connect`. Поэтому мы компилируем **один** бинарный файл.

### Requirements
1.  Определить OS (`runtime.GOOS`) для добавления `.exe` суффикса на Windows.
2.  Создать временную директорию `/tmp/revsocks_e2e_<random>/`.
3.  Выполнить `go build` для всего модуля (используя module path из `go.mod`).
4.  Вернуть путь к исполняемому файлу.
5.  **КРИТИЧНО:** Использовать абсолютный путь к корню проекта, а не относительный `../../`.

### Function Signature
```go
func BuildBinary() (binPath string, cleanup func(), err error)
```

## C. Implementation Steps

### 1. Create `tests/e2e/builder.go`
```go
package e2e

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
)

func BuildBinary() (string, func(), error) {
    // Create temp dir
    tempDir, err := os.MkdirTemp("", "revsocks_e2e")
    if err != nil {
        return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
    }

    cleanup := func() {
        os.RemoveAll(tempDir)
    }

    ext := ""
    if runtime.GOOS == "windows" {
        ext = ".exe"
    }

    binName := "revsocks-test" + ext
    binPath := filepath.Join(tempDir, binName)

    // Получаем абсолютный путь к корню проекта
    // tests/e2e находится на 2 уровня глубже корня
    projectRoot, err := filepath.Abs("../..")
    if err != nil {
        return "", cleanup, fmt.Errorf("failed to get project root: %w", err)
    }
    
    // Build с указанием рабочей директории
    cmd := exec.Command("go", "build", "-o", binPath, ".")
    cmd.Dir = projectRoot // Устанавливаем CWD в корень проекта
    
    // Capture output for debugging build failures
    if out, err := cmd.CombinedOutput(); err != nil {
        return "", cleanup, fmt.Errorf("build failed: %v\nOutput: %s", err, out)
    }

    return binPath, cleanup, nil
}
```

### 2. Update `tests/e2e/main_test.go`
Вызвать `BuildBinary` внутри `SetupEnvironment`. Сохранить путь в `GlobalCtx.BinPath`.

## D. Verification
*   **Automated:** Добавить простой тест, который проверяет наличие файла после билда.

## E. Local Checklist
```yaml
todos:
  - id: impl-builder
    content: Реализовать builder.go (single binary)
    status: pending
```

## F. Next Action
Prompt:
```text
Твоя задача — реализовать шаг 03: Process Manager.
Нужно написать обертку над `os/exec.Cmd` для запуска процессов в фоне, чтения их логов (StdoutPipe) и корректной остановки (Process.Kill).
```
