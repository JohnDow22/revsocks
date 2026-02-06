package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// ========================================
// Session Lifecycle Management
// ========================================

// ManagedSession хранит информацию о сессии агента и связанный listener
type ManagedSession struct {
	session    *yamux.Session
	listener   net.Listener
	agentID    string             // Уникальный ID агента или IP как fallback
	port       int                // Назначенный порт для SOCKS
	createdAt  time.Time          // Время создания сессии
	cancelFunc context.CancelFunc // Для остановки listenForClients
	generation uint64             // Уникальный номер сессии для защиты от race
}

// sessionGenerationCounter глобальный счётчик для generation
var sessionGenerationCounter uint64 = 0

// SessionManager управляет жизненным циклом сессий агентов
type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[string]*ManagedSession // key = agentID
	portCache map[string]int             // agentID -> последний использованный порт
}

// NewSessionManager создаёт новый менеджер сессий
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*ManagedSession),
		portCache: make(map[string]int),
	}
}

// GlobalSessionManager - глобальный менеджер сессий
var GlobalSessionManager = NewSessionManager()

// RegisterSession регистрирует новую сессию или закрывает старую с тем же agentID
// Возвращает generation для проверки в cleanup и предпочтительный порт
func (sm *SessionManager) RegisterSession(agentID string, session *yamux.Session, preferredPort int, cancelFunc context.CancelFunc) (generation uint64, port int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Инкрементируем generation
	sessionGenerationCounter++
	generation = sessionGenerationCounter

	// Проверяем есть ли уже сессия с таким agentID
	if existing, ok := sm.sessions[agentID]; ok {
		log.Printf("[%s] Closing existing session (gen %d, port %d) for new connection", agentID, existing.generation, existing.port)
		// Graceful close старой сессии
		if existing.cancelFunc != nil {
			existing.cancelFunc()
		}
		if existing.listener != nil {
			existing.listener.Close()
		}
		if existing.session != nil && !existing.session.IsClosed() {
			existing.session.Close()
		}
	}

	// Пытаемся переиспользовать порт из кэша
	port = preferredPort
	if cachedPort, ok := sm.portCache[agentID]; ok {
		port = cachedPort
		log.Printf("[%s] Reusing cached port %d", agentID, port)
	}

	sm.sessions[agentID] = &ManagedSession{
		session:    session,
		agentID:    agentID,
		port:       port,
		createdAt:  time.Now(),
		cancelFunc: cancelFunc,
		generation: generation,
	}
	sm.portCache[agentID] = port
	log.Printf("[%s] Session registered on port %d (gen %d)", agentID, port, generation)
	return generation, port
}

// SetListener устанавливает listener для сессии (вызывается из listenForClients)
// Проверяет generation чтобы не затереть новую сессию
func (sm *SessionManager) SetListener(agentID string, generation uint64, listener net.Listener) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if ms, ok := sm.sessions[agentID]; ok && ms.generation == generation {
		ms.listener = listener
		return true
	}
	return false
}

// UnregisterSession удаляет сессию и очищает ресурсы
// Проверяет generation чтобы не удалить новую сессию по ошибке
func (sm *SessionManager) UnregisterSession(agentID string, generation uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if ms, ok := sm.sessions[agentID]; ok {
		// Защита от race: удаляем только если это та же самая сессия
		if ms.generation != generation {
			log.Printf("[%s] Skipping unregister: gen mismatch (current %d, requested %d)", agentID, ms.generation, generation)
			return
		}
		log.Printf("[%s] Unregistering session (port %d, gen %d)", agentID, ms.port, generation)
		if ms.cancelFunc != nil {
			ms.cancelFunc()
		}
		if ms.listener != nil {
			ms.listener.Close()
		}
		if ms.session != nil && !ms.session.IsClosed() {
			ms.session.Close()
		}
		delete(sm.sessions, agentID)
		// НЕ удаляем из portCache чтобы при reconnect переиспользовать порт
	}
}

// GetSessionCount возвращает количество активных сессий
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// GetSocksAddr возвращает адрес SOCKS5 прокси для агента (если сессия активна)
// Возвращает пустую строку если сессия не активна
func (sm *SessionManager) GetSocksAddr(agentID string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	ms, ok := sm.sessions[agentID]
	if !ok || ms.listener == nil {
		return ""
	}
	
	// Возвращаем адрес listener'а
	return ms.listener.Addr().String()
}

// GetSessionInfo возвращает информацию о сессии агента
// Возвращает адрес SOCKS5 прокси и uptime в секундах (0 если сессия неактивна)
func (sm *SessionManager) GetSessionInfo(agentID string) (socksAddr string, uptimeSeconds int) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	ms, ok := sm.sessions[agentID]
	if !ok || ms.listener == nil {
		return "", 0
	}
	
	socksAddr = ms.listener.Addr().String()
	uptimeSeconds = int(time.Since(ms.createdAt).Seconds())
	return socksAddr, uptimeSeconds
}

// CloseSession закрывает активную сессию по agentID (для Admin API)
func (sm *SessionManager) CloseSession(agentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ms, ok := sm.sessions[agentID]
	if !ok {
		return fmt.Errorf("session not found for agent %s", agentID)
	}

	log.Printf("[%s] Closing session by admin request (port %d, gen %d)", agentID, ms.port, ms.generation)

	// Закрываем ресурсы
	if ms.cancelFunc != nil {
		ms.cancelFunc()
	}
	if ms.listener != nil {
		ms.listener.Close()
	}
	if ms.session != nil && !ms.session.IsClosed() {
		ms.session.Close()
	}

	// Удаляем из активных сессий (портCache сохраняем для reconnect)
	delete(sm.sessions, agentID)

	return nil
}

// extractAgentIP извлекает IP из remoteAddr (без порта)
func ExtractAgentIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
