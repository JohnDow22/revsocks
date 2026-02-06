package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ========================================
// Admin API Server для управления агентами
// ========================================

// AdminServer предоставляет HTTP API для управления агентами
// Слушает только localhost, авторизация не требуется
type AdminServer struct {
	manager  *AgentManager
	sessions *SessionManager // Для возможности kill активных сессий
}

// AdminAPIConfig содержит конфигурацию Admin API
type AdminAPIConfig struct {
	ListenAddr     string
	AgentManager   *AgentManager
	SessionManager *SessionManager
}

// StartAdminServer запускает HTTP сервер для Admin API
// API доступен только с localhost, авторизация не требуется
func StartAdminServer(cfg *AdminAPIConfig) error {
	srv := &AdminServer{
		manager:  cfg.AgentManager,
		sessions: cfg.SessionManager,
	}

	mux := http.NewServeMux()

	// Регистрация эндпоинтов (без авторизации, т.к. только localhost)
	mux.HandleFunc("/api/agents", srv.handleAgents)
	mux.HandleFunc("/api/agents/", srv.handleAgentConfig)
	mux.HandleFunc("/api/sessions/", srv.handleSessions)
	mux.HandleFunc("/health", srv.handleHealth)

	server := &http.Server{
		Addr:           cfg.ListenAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	log.Printf("[AdminAPI] Starting HTTP API on %s", cfg.ListenAddr)
	return server.ListenAndServe()
}


// handleHealth - healthcheck endpoint (без авторизации)
func (s *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// AgentInfoResponse содержит расширенную информацию об агенте для API
type AgentInfoResponse struct {
	*AgentConfig                   // Встраиваем базовую конфигурацию
	SocksAddr    string `json:"socks_addr,omitempty"` // Адрес SOCKS5 прокси (если активна сессия)
	IsOnline     bool   `json:"is_online"`            // Статус активной сессии
	SessionUptime int   `json:"session_uptime,omitempty"` // Время работы сессии в секундах (если активна)
}

// handleAgents обрабатывает GET /api/agents - список всех агентов
func (s *AdminServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	agents := s.manager.ListAgents()
	
	// Обогащаем информацию данными о сессиях
	response := make([]*AgentInfoResponse, 0, len(agents))
	for _, agent := range agents {
		info := &AgentInfoResponse{
			AgentConfig: agent,
			IsOnline:    false,
		}
		
		// Проверяем есть ли активная сессия
		if s.sessions != nil {
			if socksAddr, uptime := s.sessions.GetSessionInfo(agent.ID); socksAddr != "" {
				info.SocksAddr = socksAddr
				info.IsOnline = true
				info.SessionUptime = uptime
			}
		}
		
		response = append(response, info)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[AdminAPI] Error encoding agents: %v", err)
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}
}

// handleAgentConfig обрабатывает операции с конкретным агентом
// POST /api/agents/{id}/config - обновить конфигурацию
// DELETE /api/agents/{id} - удалить агента
func (s *AdminServer) handleAgentConfig(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL: /api/agents/{id}/config или /api/agents/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, `{"error": "Agent ID required"}`, http.StatusBadRequest)
		return
	}

	agentID := parts[0]

	// POST /api/agents/{id}/config - обновление конфигурации
	if r.Method == http.MethodPost && len(parts) >= 2 && parts[1] == "config" {
		s.handleUpdateAgentConfig(w, r, agentID)
		return
	}

	// DELETE /api/agents/{id} - удаление агента
	if r.Method == http.MethodDelete && len(parts) == 1 {
		s.handleDeleteAgent(w, r, agentID)
		return
	}

	http.Error(w, `{"error": "Invalid endpoint or method"}`, http.StatusBadRequest)
}

// UpdateConfigRequest структура запроса для обновления конфигурации агента
type UpdateConfigRequest struct {
	Mode          *string `json:"mode,omitempty"`           // "TUNNEL" или "SLEEP"
	SleepInterval *int    `json:"sleep_interval,omitempty"` // Интервал сна в секундах
	Jitter        *int    `json:"jitter,omitempty"`         // Jitter в процентах
	Alias         *string `json:"alias,omitempty"`          // Человекочитаемый алиас
}

// handleUpdateAgentConfig обновляет конфигурацию агента
func (s *AdminServer) handleUpdateAgentConfig(w http.ResponseWriter, r *http.Request, agentID string) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Проверяем что агент существует
	agent := s.manager.GetConfig(agentID)
	if agent == nil {
		http.Error(w, `{"error": "Agent not found"}`, http.StatusNotFound)
		return
	}

	// Обновляем алиас если указан
	if req.Alias != nil {
		if err := s.manager.UpdateAlias(agentID, *req.Alias); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
	}

	// Обновляем состояние если указаны параметры режима
	if req.Mode != nil || req.SleepInterval != nil || req.Jitter != nil {
		// Используем текущие значения если не указаны новые
		mode := agent.Mode
		if req.Mode != nil {
			mode = AgentState(*req.Mode)
		}

		sleepInterval := agent.SleepInterval
		if req.SleepInterval != nil {
			sleepInterval = *req.SleepInterval
		}

		jitter := agent.Jitter
		if req.Jitter != nil {
			jitter = *req.Jitter
		}

		// Валидация
		if mode != StateTunnel && mode != StateSleep {
			http.Error(w, `{"error": "Invalid mode: must be TUNNEL or SLEEP"}`, http.StatusBadRequest)
			return
		}

		if sleepInterval < 1 || sleepInterval > 86400 {
			http.Error(w, `{"error": "Invalid sleep_interval: must be between 1 and 86400 seconds"}`, http.StatusBadRequest)
			return
		}

		if jitter < 0 || jitter > 100 {
			http.Error(w, `{"error": "Invalid jitter: must be between 0 and 100 percent"}`, http.StatusBadRequest)
			return
		}

		if err := s.manager.UpdateState(agentID, mode, sleepInterval, jitter); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		// Принудительно разрываем активную сессию, чтобы агент
		// немедленно переподключился и применил новый режим (SLEEP/TUNNEL).
		// Игнорируем ошибку - сессии может не быть (агент уже спит или офлайн).
		if s.sessions != nil {
			if err := s.sessions.CloseSession(agentID); err == nil {
				log.Printf("[AdminAPI] Session closed for agent %s to apply new config", agentID)
			}
		}
	}

	// Возвращаем обновлённую конфигурацию
	updatedAgent := s.manager.GetConfig(agentID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedAgent)
}

// handleDeleteAgent удаляет агента из базы
func (s *AdminServer) handleDeleteAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	if err := s.manager.DeleteAgent(agentID); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "deleted",
		"agent_id": agentID,
	})
}

// handleSessions обрабатывает операции с активными сессиями
// DELETE /api/sessions/{id} - убить активную сессию
func (s *AdminServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем ID из URL: /api/sessions/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if path == "" {
		http.Error(w, `{"error": "Session ID required"}`, http.StatusBadRequest)
		return
	}

	agentID := path

	// Пытаемся убить активную сессию
	if s.sessions != nil {
		if err := s.sessions.CloseSession(agentID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, fmt.Sprintf(`{"error": "Session not found for agent %s"}`, agentID), http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "killed",
			"agent_id": agentID,
		})
		return
	}

	http.Error(w, `{"error": "SessionManager not available"}`, http.StatusServiceUnavailable)
}
