package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// ========================================
// Agent Management & State Persistence
// ========================================

// AgentState описывает режим работы агента
type AgentState string

const (
	// StateTunnel - агент поддерживает постоянное соединение (режим tunnel)
	StateTunnel AgentState = "TUNNEL"
	// StateSleep - агент периодически проверяет задачи (режим beacon/sleep)
	StateSleep AgentState = "SLEEP"
)

// AgentConfig содержит конфигурацию и состояние агента
type AgentConfig struct {
	ID            string     `json:"id"`             // Уникальный ID агента
	Alias         string     `json:"alias"`          // Человекочитаемый алиас (опционально)
	Mode          AgentState `json:"mode"`           // Текущий режим (TUNNEL/SLEEP)
	SleepInterval int        `json:"sleep_interval"` // Интервал сна в секундах
	Jitter        int        `json:"jitter"`         // Jitter в процентах (0-100)
	LastSeen      time.Time  `json:"last_seen"`      // Последний раз когда агент подключался
	FirstSeen     time.Time  `json:"first_seen"`     // Первое подключение агента
	IP            string     `json:"ip"`             // Последний известный IP
	Version       string     `json:"version"`        // Версия агента (если передана)
}

// AgentManager управляет состоянием агентов и персистентностью данных
type AgentManager struct {
	mu     sync.RWMutex
	agents map[string]*AgentConfig // key = agent ID
	dbPath string                  // Путь к JSON файлу
}

// NewAgentManager создаёт новый менеджер агентов
func NewAgentManager(path string) (*AgentManager, error) {
	am := &AgentManager{
		agents: make(map[string]*AgentConfig),
		dbPath: path,
	}

	// Пытаемся загрузить существующую БД
	err := am.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Failed to load agent database from %s: %v", path, err)
	}

	return am, nil
}

// Load загружает конфигурацию агентов из JSON файла
func (am *AgentManager) Load() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(am.dbPath)
	if err != nil {
		return err
	}

	var agents []*AgentConfig
	if err := json.Unmarshal(data, &agents); err != nil {
		return fmt.Errorf("failed to unmarshal agents: %w", err)
	}

	// Восстанавливаем map
	am.agents = make(map[string]*AgentConfig)
	for _, agent := range agents {
		am.agents[agent.ID] = agent
	}

	log.Printf("Loaded %d agents from %s", len(agents), am.dbPath)
	return nil
}

// Save сохраняет текущую конфигурацию агентов в JSON файл
func (am *AgentManager) Save() error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Конвертируем map в slice для JSON
	agents := make([]*AgentConfig, 0, len(am.agents))
	for _, agent := range am.agents {
		agents = append(agents, agent)
	}

	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agents: %w", err)
	}

	if err := os.WriteFile(am.dbPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write agents to %s: %w", am.dbPath, err)
	}

	return nil
}

// GetConfig возвращает конфигурацию агента по ID (или nil если не найдено)
func (am *AgentManager) GetConfig(id string) *AgentConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agent, ok := am.agents[id]
	if !ok {
		return nil
	}

	// Возвращаем копию для безопасности
	agentCopy := *agent
	return &agentCopy
}

// RegisterAgent регистрирует новый агент или обновляет LastSeen для существующего
// Возвращает конфигурацию агента
func (am *AgentManager) RegisterAgent(id, ip, version string) (*AgentConfig, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()

	agent, exists := am.agents[id]
	if !exists {
		// Создаём нового агента с дефолтными параметрами
		agent = &AgentConfig{
			ID:            id,
			Alias:         "", // Пустой по умолчанию
			Mode:          StateTunnel,
			SleepInterval: 60,  // 60 секунд по умолчанию
			Jitter:        10,  // 10% jitter по умолчанию
			FirstSeen:     now,
			LastSeen:      now,
			IP:            ip,
			Version:       version,
		}
		am.agents[id] = agent
		log.Printf("[AgentManager] New agent registered: %s (IP: %s, Version: %s)", id, ip, version)
	} else {
		// Обновляем LastSeen, IP и версию
		agent.LastSeen = now
		agent.IP = ip
		if version != "" && version != "unknown" {
			agent.Version = version
		}
		log.Printf("[AgentManager] Agent check-in: %s (IP: %s, Mode: %s, Version: %s)", id, ip, agent.Mode, agent.Version)
	}

	// Асинхронно сохраняем в JSON
	go func() {
		if err := am.Save(); err != nil {
			log.Printf("[AgentManager] Error saving agents: %v", err)
		}
	}()

	// Возвращаем копию
	agentCopy := *agent
	return &agentCopy, nil
}

// UpdateState обновляет состояние агента
func (am *AgentManager) UpdateState(id string, mode AgentState, interval, jitter int) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agent, ok := am.agents[id]
	if !ok {
		return fmt.Errorf("agent %s not found", id)
	}

	agent.Mode = mode
	agent.SleepInterval = interval
	agent.Jitter = jitter

	// Асинхронно сохраняем
	go func() {
		if err := am.Save(); err != nil {
			log.Printf("[AgentManager] Error saving agents: %v", err)
		}
	}()

	log.Printf("[AgentManager] Agent %s updated: Mode=%s, Interval=%d, Jitter=%d", id, mode, interval, jitter)
	return nil
}

// UpdateAlias обновляет человекочитаемый алиас агента
func (am *AgentManager) UpdateAlias(id, alias string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agent, ok := am.agents[id]
	if !ok {
		return fmt.Errorf("agent %s not found", id)
	}

	agent.Alias = alias

	// Асинхронно сохраняем
	go func() {
		if err := am.Save(); err != nil {
			log.Printf("[AgentManager] Error saving agents: %v", err)
		}
	}()

	log.Printf("[AgentManager] Agent %s alias updated to '%s'", id, alias)
	return nil
}

// ListAgents возвращает копии всех агентов
func (am *AgentManager) ListAgents() []*AgentConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agents := make([]*AgentConfig, 0, len(am.agents))
	for _, agent := range am.agents {
		agentCopy := *agent
		agents = append(agents, &agentCopy)
	}

	return agents
}

// DeleteAgent удаляет агента из базы
func (am *AgentManager) DeleteAgent(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, ok := am.agents[id]; !ok {
		return fmt.Errorf("agent %s not found", id)
	}

	delete(am.agents, id)

	// Асинхронно сохраняем
	go func() {
		if err := am.Save(); err != nil {
			log.Printf("[AgentManager] Error saving agents: %v", err)
		}
	}()

	log.Printf("[AgentManager] Agent %s deleted", id)
	return nil
}
