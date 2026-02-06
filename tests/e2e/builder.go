package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Binaries содержит пути к собранным бинарникам
type Binaries struct {
	AgentPath  string
	ServerPath string
	TempDir    string
}

// BuildBinaries компилирует оба бинарника (agent и server) во временную директорию
// Возвращает структуру с путями к бинарникам и cleanup функцию
func BuildBinaries() (*Binaries, func(), error) {
	// Создаём временную директорию
	tempDir, err := os.MkdirTemp("", "revsocks_e2e")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Определяем расширение для Windows
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// Получаем абсолютный путь к корню проекта
	// tests/e2e находится на 2 уровня глубже корня
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed to get project root: %w", err)
	}

	binaries := &Binaries{
		TempDir:    tempDir,
		AgentPath:  filepath.Join(tempDir, "revsocks-agent"+ext),
		ServerPath: filepath.Join(tempDir, "revsocks-server"+ext),
	}

	// Компилируем agent
	agentCmd := exec.Command("go", "build", "-o", binaries.AgentPath, "./cmd/agent")
	agentCmd.Dir = projectRoot
	output, err := agentCmd.CombinedOutput()
	if err != nil {
		return nil, cleanup, fmt.Errorf("agent build failed: %w\nOutput: %s", err, string(output))
	}

	// Проверяем что agent создан
	if _, err := os.Stat(binaries.AgentPath); err != nil {
		return nil, cleanup, fmt.Errorf("agent binary not found after build: %w", err)
	}

	// Компилируем server
	serverCmd := exec.Command("go", "build", "-o", binaries.ServerPath, "./cmd/server")
	serverCmd.Dir = projectRoot
	output, err = serverCmd.CombinedOutput()
	if err != nil {
		return nil, cleanup, fmt.Errorf("server build failed: %w\nOutput: %s", err, string(output))
	}

	// Проверяем что server создан
	if _, err := os.Stat(binaries.ServerPath); err != nil {
		return nil, cleanup, fmt.Errorf("server binary not found after build: %w", err)
	}

	return binaries, cleanup, nil
}

// BuildBinary компилирует RevSocks во временную директорию
// DEPRECATED: Используйте BuildBinaries() для новой архитектуры
// Сохранено для обратной совместимости - возвращает путь к agent
func BuildBinary() (string, func(), error) {
	binaries, cleanup, err := BuildBinaries()
	if err != nil {
		return "", cleanup, err
	}
	return binaries.AgentPath, cleanup, nil
}
