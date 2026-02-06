package e2e

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Process - обёртка над os/exec.Cmd для управления процессами RevSocks
type Process struct {
	BinPath string
	Name    string

	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	mu     sync.Mutex // Защита буферов от гонок
}

// NewProcess создаёт новый процесс
func NewProcess(binPath, name string) *Process {
	return &Process{
		BinPath: binPath,
		Name:    name,
	}
}

// Start запускает процесс с указанными аргументами
func (p *Process) Start(args ...string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return fmt.Errorf("process %s already started", p.Name)
	}

	p.cmd = exec.Command(p.BinPath, args...)
	p.cmd.Stdout = &p.stdout
	p.cmd.Stderr = &p.stderr

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", p.Name, err)
	}

	return nil
}

// Stop останавливает процесс
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil // Процесс не запущен
	}

	// Убиваем процесс
	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill %s: %w", p.Name, err)
	}

	// Ждём завершения
	_ = p.cmd.Wait()

	return nil
}

// WaitForLog ожидает появления подстроки в логах с таймаутом
func (p *Process) WaitForLog(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		p.mu.Lock()
		output := p.stdout.String() + p.stderr.String()
		p.mu.Unlock()

		if strings.Contains(output, pattern) {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for log pattern %q in %s", pattern, p.Name)
}

// GetOutput возвращает объединённый вывод для отладки
func (p *Process) GetOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return fmt.Sprintf("=== STDOUT ===\n%s\n=== STDERR ===\n%s",
		p.stdout.String(), p.stderr.String())
}

// IsRunning проверяет что процесс запущен
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	// Проверяем статус процесса
	// Process.Signal(nil) возвращает ошибку если процесс завершён
	return p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()
}
