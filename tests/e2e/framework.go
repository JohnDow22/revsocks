package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TestContext хранит глобальные настройки для E2E тестов
type TestContext struct {
	// BinPath - путь к скомпилированному бинарнику agent (для обратной совместимости)
	BinPath string

	// AgentPath - путь к бинарнику revsocks-agent
	AgentPath string

	// ServerPath - путь к бинарнику revsocks-server
	ServerPath string
}

// GlobalCtx - глобальный контекст, заполняется в TestMain
var GlobalCtx TestContext

// ========================================
// Admin API Helpers
// ========================================

// GetAgents получает список агентов через Admin API (localhost, без авторизации)
func GetAgents(adminAddr, token string) ([]map[string]interface{}, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/api/agents", adminAddr), nil)
	if err != nil {
		return nil, err
	}
	// token не используется (API без авторизации), но сохраняем сигнатуру для совместимости

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var agents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}

	return agents, nil
}

// UpdateAgentConfig обновляет конфигурацию агента через Admin API (localhost, без авторизации)
func UpdateAgentConfig(adminAddr, token, agentID string, config map[string]interface{}) error {
	client := &http.Client{Timeout: 5 * time.Second}

	body, err := json.Marshal(config)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/api/agents/%s/config", adminAddr, agentID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	// token не используется (API без авторизации), но сохраняем сигнатуру для совместимости
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
