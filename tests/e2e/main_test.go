package e2e

import (
	"fmt"
	"os"
	"testing"
)

// TestMain выполняется один раз перед всеми тестами
// Компилирует бинарники RevSocks и настраивает окружение
func TestMain(m *testing.M) {
	// Setup: Компилируем бинарники
	if err := SetupEnvironment(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup environment: %v\n", err)
		os.Exit(1)
	}

	// Запускаем тесты
	code := m.Run()

	// Teardown: Очистка
	TeardownEnvironment()

	os.Exit(code)
}

// cleanupFunc хранит функцию очистки временных файлов
var cleanupFunc func()

// isExecutable проверяет, что файл существует и имеет права на исполнение.
func isExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}
	// Для Linux достаточно проверять x-бит.
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("file is not executable: %s", path)
	}
	return nil
}

// SetupEnvironment подготавливает окружение для тестов
func SetupEnvironment() error {
	// Если пользователь дал готовые бинарники — используем их и не компилируем.
	// Это удобно для тестирования "как в проде" (реально собранных артефактов).
	serverBin := os.Getenv("TEST_SERVER_BIN")
	agentBin := os.Getenv("TEST_AGENT_BIN")
	if serverBin != "" || agentBin != "" {
		if serverBin == "" || agentBin == "" {
			return fmt.Errorf("для режима внешних бинарников нужно задать ОБЕ переменные: TEST_SERVER_BIN и TEST_AGENT_BIN")
		}
		if err := isExecutable(serverBin); err != nil {
			return fmt.Errorf("некорректный TEST_SERVER_BIN: %w", err)
		}
		if err := isExecutable(agentBin); err != nil {
			return fmt.Errorf("некорректный TEST_AGENT_BIN: %w", err)
		}

		GlobalCtx.ServerPath = serverBin
		GlobalCtx.AgentPath = agentBin
		GlobalCtx.BinPath = agentBin // для обратной совместимости
		cleanupFunc = nil

		fmt.Printf("✅ Using external server binary: %s\n", serverBin)
		fmt.Printf("✅ Using external agent binary: %s\n", agentBin)
		return nil
	}

	// Компилируем оба бинарника
	binaries, cleanup, err := BuildBinaries()
	if err != nil {
		return fmt.Errorf("failed to build binaries: %w", err)
	}

	// Сохраняем пути к бинарникам в глобальном контексте
	GlobalCtx.AgentPath = binaries.AgentPath
	GlobalCtx.ServerPath = binaries.ServerPath
	GlobalCtx.BinPath = binaries.AgentPath // для обратной совместимости
	cleanupFunc = cleanup

	fmt.Printf("✅ Built agent: %s\n", binaries.AgentPath)
	fmt.Printf("✅ Built server: %s\n", binaries.ServerPath)

	return nil
}

// TeardownEnvironment очищает временные файлы
func TeardownEnvironment() {
	if cleanupFunc != nil {
		cleanupFunc()
	}
}
