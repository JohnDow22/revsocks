package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/acme/autocert"
	"nhooyr.io/websocket"

	"github.com/kost/revsocks/internal/common"
	"github.com/kost/revsocks/internal/transport"
)

// Config содержит настройки сервера
type Config struct {
	// Сетевые параметры
	ListenAddress string // Адрес для агентов (host:port)
	ClientsListen string // Адрес для SOCKS клиентов (host:port)

	// TLS
	UseTLS         bool   // Использовать TLS
	Certificate    string // Путь к сертификату
	AutocertDomain string // Домен для автоматического получения сертификата

	// Аутентификация
	Password string // Пароль для агентов

	// Timeouts
	ProxyTimeout time.Duration

	// Agent Management
	AgentManager *AgentManager // Менеджер состояний агентов
}

// agentHandler обрабатывает WebSocket соединения от агентов
type agentHandler struct {
	mu           sync.Mutex
	listenstr    string        // listen string for clients
	portnext     int           // next port for listen
	timeout      time.Duration
	password     string        // пароль для аутентификации
	agentManager *AgentManager // менеджер агентов для персистентности
}

func (h *agentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var session *yamux.Session
	var erry error

	agentstr := r.RemoteAddr

	// Проверка WebSocket upgrade
	if r.Header.Get("Upgrade") != "websocket" {
		// Тихий редирект без логирования (защита от флуда сканерами)
		common.DebugLog("[%s] Non-WS request (%s): %s -> redirect", agentstr, r.Method, r.URL.String())
		w.Header().Set("Location", "https://www.microsoft.com/")
		w.WriteHeader(http.StatusFound)
		return
	}

	// Проверка пароля
	if r.Header.Get("Accept-Language") != h.password {
		common.DebugLog("[%s] Invalid password in WS request -> redirect", agentstr)
		w.Header().Set("Location", "https://www.microsoft.com/")
		w.WriteHeader(http.StatusFound)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("[%s] Error upgrading to socket (%s):  %v", agentstr, r.RemoteAddr, err)
		http.Error(w, "Bad request - Go away!", 500)
		return
	}
	defer c.CloseNow()

	// Извлекаем agent_id из кастомного заголовка или используем IP как fallback
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = ExtractAgentIP(agentstr)
	}

	// Извлекаем версию агента из заголовка
	agentVersion := r.Header.Get("X-Agent-Version")
	if agentVersion == "" {
		agentVersion = "unknown"
	}

	if h.timeout > 0 {
		_, cancel := context.WithTimeout(r.Context(), time.Second*60)
		defer cancel()
	}

	log.Printf("[%s] Got Client via WebSocket (agentID: %s, version: %s)", agentstr, agentID, agentVersion)

	// Регистрируем агента в AgentManager для персистентности
	var agentConfig *AgentConfig
	if h.agentManager != nil {
		agentIP := ExtractAgentIP(agentstr)
		agentConfig, err = h.agentManager.RegisterAgent(agentID, agentIP, agentVersion)
		if err != nil {
			log.Printf("[%s] Warning: failed to register agent in AgentManager: %v", agentID, err)
			// Не фатальная ошибка - продолжаем в TUNNEL режиме
		}
	}

	// === Протокол v3: отправляем команду агенту через WebSocket ===
	// Определяем режим агента
	agentMode := StateTunnel
	sleepInterval := 60
	jitter := 10
	if agentConfig != nil {
		agentMode = agentConfig.Mode
		sleepInterval = agentConfig.SleepInterval
		jitter = agentConfig.Jitter
	}

	// Отправляем команду в зависимости от режима
	ctx := r.Context()
	switch agentMode {
	case StateSleep:
		// Sleep режим: отправляем команду и закрываем соединение
		cmd := fmt.Sprintf("%s %d %d", common.CmdSleep, sleepInterval, jitter)
		log.Printf("[%s] Agent %s mode: SLEEP (%d sec, %d%% jitter)", agentstr, agentID, sleepInterval, jitter)
		if err := c.Write(ctx, websocket.MessageText, []byte(cmd+"\n")); err != nil {
			log.Printf("[%s] Failed to send SLEEP command: %v", agentstr, err)
			return
		}
		// Закрываем соединение - агент должен спать
		c.Close(websocket.StatusNormalClosure, "sleep mode")
		return

	default:
		// TUNNEL режим (или неизвестный): отправляем команду и продолжаем
		log.Printf("[%s] Agent %s mode: TUNNEL", agentstr, agentID)
		if err := c.Write(ctx, websocket.MessageText, []byte(common.CmdTunnel+"\n")); err != nil {
			log.Printf("[%s] Failed to send TUNNEL command: %v", agentstr, err)
			return
		}
	}

	// === Создаём yamux сессию (только для TUNNEL режима) ===
	nc_over_ws := websocket.NetConn(context.Background(), c, websocket.MessageBinary)

	session, erry = yamux.Client(nc_over_ws, transport.GetYamuxConfig())
	if erry != nil {
		log.Printf("[%s] Error creating client in yamux for (%s): %v", agentID, r.RemoteAddr, erry)
		http.Error(w, "Bad request - Go away!", 500)
		return
	}

	h.mu.Lock()
	preferredPort := h.portnext
	h.portnext = h.portnext + 1
	h.mu.Unlock()

	// Создаём context для lifecycle management
	sessionCtx, cancel := context.WithCancel(context.Background())

	// Регистрируем сессию в SessionManager
	generation, assignedPort := GlobalSessionManager.RegisterSession(agentID, session, preferredPort, cancel)

	listenForClients(sessionCtx, agentID, h.listenstr, assignedPort, session, generation)

	// Cleanup при выходе (с проверкой generation)
	GlobalSessionManager.UnregisterSession(agentID, generation)
	c.Close(websocket.StatusNormalClosure, "")
}

// ListenWebsocket запускает сервер для WebSocket агентов
func ListenWebsocket(cfg *Config) error {
	var cer tls.Certificate
	var err error
	log.Printf("Will start listening for clients on %s", cfg.ClientsListen)

	host, portStr, err := net.SplitHostPort(cfg.ClientsListen)
	if err != nil {
		log.Fatalf("Invalid client listen address '%s': %v", cfg.ClientsListen, err)
	}
	portnum, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid port in '%s': %v", cfg.ClientsListen, err)
	}

	aHandler := &agentHandler{
		portnext:     portnum,
		listenstr:    host,
		password:     cfg.Password,
		agentManager: cfg.AgentManager,
	}
	server := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: aHandler,
	}

	if cfg.UseTLS {
		if cfg.AutocertDomain != "" {
			log.Printf("Getting TLS certificate for %s", cfg.AutocertDomain)
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Printf("Error getting TLS certificate for %s: %v", cfg.AutocertDomain, err)
			}
			cachepath := filepath.Join(dirname, ".revsocks-autocert")
			m := &autocert.Manager{
				Cache:      autocert.DirCache(cachepath),
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(cfg.AutocertDomain),
			}
			server.TLSConfig = m.TLSConfig()
		} else {
			if cfg.Certificate == "" {
				cer, err = transport.GetCachedTLS(2048)
				log.Println("Using cached/generated TLS certificate.")
			} else {
				cer, err = tls.LoadX509KeyPair(cfg.Certificate+".crt", cfg.Certificate+".key")
			}
			if err != nil {
				log.Printf("Error creating/loading certificate file %s: %v", cfg.Certificate, err)
				return err
			}
			server.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cer},
			}
		}
	}

	log.Printf("Listening for websocket agents on %s (TLS: %t)", cfg.ListenAddress, cfg.UseTLS)
	if cfg.UseTLS {
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}

	return err
}

// ========================================
// Handshake v3 Protocol
// ========================================

// parseHandshakeV3 читает handshake v3: "AUTH <password> <agent_id> <version> <yamux_cfg>\n"
// Возвращает agentID, version, yamuxSettings и ошибку если парсинг не удался
// Сервер ПРИНИМАЕТ настройки клиента и использует их для yamux сессии (синхронизация)
func parseHandshakeV3(reader *bufio.Reader, cfg *Config) (string, string, *transport.YamuxSettings, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read auth line: %w", err)
	}

	line = strings.TrimSpace(line)
	parts := strings.Fields(line) // Split by whitespace

	// Минимум 5 полей: AUTH <password> <agent_id> <version> <yamux_cfg>
	if len(parts) < 5 {
		return "", "", nil, fmt.Errorf("invalid handshake format: expected 'AUTH <password> <agent_id> <version> <yamux_cfg>', got: %s", line)
	}

	if parts[0] != "AUTH" {
		return "", "", nil, fmt.Errorf("invalid handshake: expected 'AUTH', got '%s'", parts[0])
	}

	password := parts[1]
	agentID := parts[2]
	version := parts[3]

	// Проверка пароля
	if password != cfg.Password {
		return "", "", nil, fmt.Errorf("authentication failed: password mismatch")
	}

	// Парсим yamux настройки клиента
	yamuxCfgStr := parts[4]
	clientSettings, err := transport.ParseYamuxHandshake(yamuxCfgStr)
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid yamux config: %v", err)
	}

	log.Printf("[%s] Yamux config from client: keepalive=%ds, timeout=%ds, enabled=%t",
		agentID,
		int(clientSettings.KeepAliveInterval.Seconds()),
		int(clientSettings.WriteTimeout.Seconds()),
		clientSettings.EnableKeepAlive)

	return agentID, version, clientSettings, nil
}

// sendCommand отправляет команду клиенту с переводом строки
func sendCommand(conn net.Conn, cmd string) error {
	_, err := conn.Write([]byte(cmd + "\n"))
	return err
}

// handleConnectionV3 обрабатывает handshake v3 и решает что делать с агентом
// Возвращает agentID, yamuxSettings для использования в yamux сессии, и ошибку
// При SLEEP режиме возвращает специальную ошибку ErrAgentSleep
func handleConnectionV3(conn net.Conn, cfg *Config, agentstr string, reader *bufio.Reader) (string, *transport.YamuxSettings, error) {
	// Парсим handshake
	agentID, version, yamuxSettings, err := parseHandshakeV3(reader, cfg)
	if err != nil {
		log.Printf("[%s] Handshake v3 failed: %v", agentstr, err)
		sendCommand(conn, common.AuthFail)
		return "", nil, err
	}

	log.Printf("[%s] Handshake v3 successful: agentID=%s, version=%s", agentstr, agentID, version)

	// Регистрируем агента в AgentManager
	agentConfig, err := cfg.AgentManager.RegisterAgent(agentID, ExtractAgentIP(agentstr), version)
	if err != nil {
		log.Printf("[%s] Failed to register agent: %v", agentstr, err)
		sendCommand(conn, "ERR Internal Error")
		return "", nil, err
	}

	// Определяем действие на основе режима агента
	switch agentConfig.Mode {
	case StateTunnel:
		// Tunnel режим: отправляем команду и запускаем yamux
		log.Printf("[%s] Agent %s mode: TUNNEL", agentstr, agentID)
		if err := sendCommand(conn, common.CmdTunnel); err != nil {
			log.Printf("[%s] Failed to send TUNNEL command: %v", agentstr, err)
			return "", nil, err
		}
		return agentID, yamuxSettings, nil // Caller продолжит с yamux

	case StateSleep:
		// Sleep режим: отправляем команду и закрываем соединение
		cmd := fmt.Sprintf("%s %d %d", common.CmdSleep, agentConfig.SleepInterval, agentConfig.Jitter)
		log.Printf("[%s] Agent %s mode: SLEEP (%d sec, %d%% jitter)", agentstr, agentID, agentConfig.SleepInterval, agentConfig.Jitter)
		if err := sendCommand(conn, cmd); err != nil {
			log.Printf("[%s] Failed to send SLEEP command: %v", agentstr, err)
			return "", nil, err
		}
		// Закрываем соединение - агент должен спать
		conn.Close()
		return "", nil, fmt.Errorf("agent in SLEEP mode, connection closed")

	default:
		log.Printf("[%s] Unknown agent mode: %s, falling back to TUNNEL", agentstr, agentConfig.Mode)
		sendCommand(conn, common.CmdTunnel)
		return agentID, yamuxSettings, nil
	}
}

// Listen запускает сервер для TCP агентов
func Listen(cfg *Config) error {
	var err, erry error
	var cer tls.Certificate
	var session *yamux.Session
	var ln net.Listener

	log.Printf("Will start listening for clients on %s and agents on %s (TLS: %t)", cfg.ClientsListen, cfg.ListenAddress, cfg.UseTLS)

	// Валидация длины пароля
	if len(cfg.Password) > 64 {
		return fmt.Errorf("password too long: max 64 bytes, got %d", len(cfg.Password))
	}

	if cfg.UseTLS {
		if cfg.AutocertDomain != "" {
			log.Printf("Getting TLS certificate for %s", cfg.AutocertDomain)
			dirname, err := os.UserHomeDir()
			if err != nil {
				log.Printf("Error getting TLS certificate for %s: %v", cfg.AutocertDomain, err)
			}
			cachepath := filepath.Join(dirname, ".revsocks-autocert")
			m := &autocert.Manager{
				Cache:      autocert.DirCache(cachepath),
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(cfg.AutocertDomain),
			}
			ln, err = tls.Listen("tcp", cfg.ListenAddress, m.TLSConfig())
		} else {
			if cfg.Certificate == "" {
				cer, err = transport.GetCachedTLS(2048)
				log.Println("Using cached/generated TLS certificate.")
			} else {
				cer, err = tls.LoadX509KeyPair(cfg.Certificate+".crt", cfg.Certificate+".key")
			}
			if err != nil {
				log.Println(err)
				return err
			}
			config := &tls.Config{Certificates: []tls.Certificate{cer}}
			ln, err = tls.Listen("tcp", cfg.ListenAddress, config)
		}
	} else {
		ln, err = net.Listen("tcp", cfg.ListenAddress)
	}
	if err != nil {
		log.Printf("Error listening on %s: %v", cfg.ListenAddress, err)
		return err
	}

	host, portStr, err := net.SplitHostPort(cfg.ClientsListen)
	if err != nil {
		log.Fatalf("Invalid client listen address '%s': %v", cfg.ClientsListen, err)
	}
	portnum, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid port in '%s': %v", cfg.ClientsListen, err)
	}

	portinc := 0
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Errors accepting!")
			continue
		}
		agentstr := conn.RemoteAddr().String()
		common.DebugLog("[%s] Got a connection from %v", agentstr, conn.RemoteAddr())

		reader := bufio.NewReader(conn)

		// Пытаемся прочитать первую строку для определения протокола
		conn.SetReadDeadline(time.Now().Add(cfg.ProxyTimeout))

		// Peek первые байты чтобы определить протокол (v2 или v3)
		firstBytes, err := reader.Peek(4)
		if err != nil {
			log.Printf("[%s] Error peeking connection: %v", agentstr, err)
			conn.Close()
			continue
		}

		// Проверяем что это v3 protocol (начинается с "AUTH")
		if string(firstBytes) != "AUTH" {
			// Неизвестный протокол - отвечаем редиректом (скрываем сервер)
			common.DebugLog("[%s] Unknown protocol, sending redirect", agentstr)
			httpresonse := "HTTP/1.1 301 Moved Permanently" +
				"\r\nContent-Type: text/html; charset=UTF-8" +
				"\r\nLocation: https://www.microsoft.com/" +
				"\r\nServer: Apache" +
				"\r\nContent-Length: 0" +
				"\r\nConnection: close" +
				"\r\n\r\n"
			conn.Write([]byte(httpresonse))
			conn.Close()
			continue
		}

		// ========================================
		// Handshake v3 Protocol (единственный поддерживаемый)
		// ========================================
		agentID, yamuxSettings, err := handleConnectionV3(conn, cfg, agentstr, reader)
		if err != nil {
			// Если ошибка (включая SLEEP режим) - соединение уже закрыто в handleConnectionV3
			log.Printf("[%s] Handshake v3 completed: %v", agentstr, err)
			continue
		}

		// Если успешно и режим TUNNEL - продолжаем с yamux
		conn.SetReadDeadline(time.Time{}) // Сброс deadline

		// Создаём yamux сессию с настройками КЛИЕНТА (синхронизация)
		session, erry = yamux.Client(conn, transport.NewYamuxConfig(yamuxSettings))
		if erry != nil {
			log.Printf("[%s] Error creating client in yamux: %v", agentstr, erry)
			conn.Close()
			continue
		}

		// agentID теперь корректно передан из handleConnectionV3
		log.Printf("[%s] Creating session for agent: %s", agentstr, agentID)

		// Создаём context для lifecycle management
		ctx, cancel := context.WithCancel(context.Background())
		preferredPort := portnum + portinc

		generation, assignedPort := GlobalSessionManager.RegisterSession(agentID, session, preferredPort, cancel)

		go listenForClients(ctx, agentID, host, assignedPort, session, generation)
		portinc = portinc + 1
	}
}

// listenForClients принимает подключения от SOCKS клиентов и связывает с yamux
func listenForClients(ctx context.Context, agentID string, listen string, port int, session *yamux.Session, generation uint64) error {
	var ln net.Listener
	var address string
	var err error
	portinc := port

	for {
		address = fmt.Sprintf("%s:%d", listen, portinc)
		log.Printf("[%s] Handshake recognized. Waiting for clients on %s (gen %d)", agentID, address, generation)
		ln, err = net.Listen("tcp", address)
		if err != nil {
			log.Printf("[%s] Error listening on %s: %v", agentID, address, err)
			portinc = portinc + 1
			if portinc > port+100 {
				log.Printf("[%s] Failed to find available port after 100 attempts", agentID)
				return fmt.Errorf("no available port")
			}
		} else {
			break
		}
	}

	// Регистрируем listener в SessionManager с проверкой generation
	if !GlobalSessionManager.SetListener(agentID, generation, ln) {
		log.Printf("[%s] Session was replaced, closing listener (gen %d)", agentID, generation)
		ln.Close()
		return fmt.Errorf("session replaced")
	}

	// Горутина для мониторинга состояния сессии и context
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("[%s] Context cancelled, closing listener on %s (gen %d)", agentID, address, generation)
				ln.Close()
				return
			case <-ticker.C:
				if session.IsClosed() {
					log.Printf("[%s] Session closed, stopping listener on %s (gen %d)", agentID, address, generation)
					ln.Close()
					GlobalSessionManager.UnregisterSession(agentID, generation)
					return
				}
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Printf("[%s] Accept stopped due to context cancellation", agentID)
				return nil
			default:
				if session.IsClosed() {
					log.Printf("[%s] Session closed, stopping accept loop on %s", agentID, address)
					return nil
				}
				log.Printf("[%s] Error accepting on %s: %v", agentID, address, err)
				return err
			}
		}
		if session == nil || session.IsClosed() {
			log.Printf("[%s] Session on %s is nil or closed", agentID, address)
			conn.Close()
			return fmt.Errorf("session closed")
		}
		log.Printf("[%s] Got client. Opening stream for %s", agentID, conn.RemoteAddr())

		stream, err := session.Open()
		if err != nil {
			log.Printf("[%s] Error opening stream for %s: %v", agentID, conn.RemoteAddr(), err)
			conn.Close()
			if session.IsClosed() {
				log.Printf("[%s] Session closed after stream error, exiting", agentID)
				return err
			}
			continue
		}

		go func(c net.Conn, s net.Conn) {
			common.DebugLog("[%s] Starting to copy conn to stream for %s", agentID, c.RemoteAddr())
			io.Copy(c, s)
			c.Close()
			common.DebugLog("[%s] Done copying conn to stream for %s", agentID, c.RemoteAddr())
		}(conn, stream)
		go func(c net.Conn, s net.Conn) {
			common.DebugLog("[%s] Starting to copy stream to conn for %s", agentID, c.RemoteAddr())
			io.Copy(s, c)
			s.Close()
			common.DebugLog("[%s] Done copying stream to conn for %s", agentID, c.RemoteAddr())
		}(conn, stream)
	}
}
