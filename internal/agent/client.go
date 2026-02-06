package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	ntlmssp "github.com/kost/go-ntlmssp"
	"nhooyr.io/websocket"

	"github.com/kost/revsocks/internal/common"
	"github.com/kost/revsocks/internal/transport"
)

var encBase64 = base64.StdEncoding.EncodeToString
var decBase64 = base64.StdEncoding.DecodeString

// Config содержит все настройки для агента
type Config struct {
	// Сетевые параметры
	Connect string // Адрес сервера (host:port или https://host:port для ws)
	Proxy   string // Прокси сервер (опционально)

	// Аутентификация
	Password  string // Пароль для подключения к серверу
	ProxyAuth *ProxyAuthConfig

	// TLS
	UseTLS bool // Использовать TLS
	Verify bool // Проверять сертификат

	// WebSocket
	UseWebsocket bool   // Использовать WebSocket
	UserAgent    string // User-Agent для HTTP запросов

	// Timeouts
	ProxyTimeout time.Duration

	// SOCKS5 Authentication (опционально)
	SocksAuthEnabled bool
	SocksAuthUser    string
	SocksAuthPass    string

	// Agent ID для дедупликации сессий на сервере
	AgentID string

	// Debug
	Debug bool
}

// ProxyAuthConfig содержит настройки аутентификации прокси
type ProxyAuthConfig struct {
	Domain   string
	Username string
	Password string
}

// sanitizeProxyConnect убирает credentials из CONNECT строки для безопасного логирования
func sanitizeProxyConnect(s string) string {
	lines := strings.Split(s, "\r\n")
	var safe []string
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "proxy-authorization:") {
			safe = append(safe, "Proxy-Authorization: [REDACTED]")
		} else {
			safe = append(safe, line)
		}
	}
	return strings.Join(safe, "\r\n")
}

// getAgentID возвращает agent ID для handshake
// Приоритет: config → hostname → случайный
func getAgentID(cfg *Config) string {
	if cfg.AgentID != "" {
		return cfg.AgentID
	}
	// Fallback на hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	// Fallback на случайный ID
	return common.RandString(16)
}

// LoadOrGenerateAgentID загружает или генерирует persistent Agent ID
// ID сохраняется в файле для переиспользования между запусками
func LoadOrGenerateAgentID(idPath string) (string, error) {
	// Если путь пустой, используем дефолтное местоположение
	if idPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback на текущую директорию
			idPath = ".revsocks.id"
		} else {
			idPath = homeDir + "/.revsocks.id"
		}
	}

	// Пытаемся прочитать существующий ID
	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" && len(id) <= common.MaxAgentIDLength {
			log.Printf("Loaded agent ID from %s: %s", idPath, id)
			return id, nil
		}
		log.Printf("Warning: Invalid agent ID in %s, generating new one", idPath)
	}

	// Генерируем новый ID (hostname или случайная строка)
	var newID string
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Используем hostname как базу, но ограничиваем длину
		if len(hostname) > common.MaxAgentIDLength {
			newID = hostname[:common.MaxAgentIDLength]
		} else {
			newID = hostname
		}
	} else {
		// Случайный ID
		newID = common.RandString(16)
	}

	// Сохраняем в файл
	err = os.WriteFile(idPath, []byte(newID+"\n"), 0600)
	if err != nil {
		log.Printf("Warning: Failed to save agent ID to %s: %v", idPath, err)
		// Не критично, продолжаем работать
	} else {
		log.Printf("Generated and saved new agent ID to %s: %s", idPath, newID)
	}

	return newID, nil
}

// getSocksConfig возвращает конфигурацию SOCKS5 сервера
func getSocksConfig(cfg *Config) *socks5.Config {
	if cfg.SocksAuthEnabled && cfg.SocksAuthUser != "" && cfg.SocksAuthPass != "" {
		creds := socks5.StaticCredentials{
			cfg.SocksAuthUser: cfg.SocksAuthPass,
		}
		return &socks5.Config{
			Credentials: creds,
		}
	}
	return &socks5.Config{}
}

// GetSystemProxy получает URL прокси из переменных окружения
func GetSystemProxy(method string, urlstr string) (*url.URL, error) {
	req, err := http.NewRequest(method, urlstr, nil)
	if err != nil {
		return nil, err
	}
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		return nil, err
	}
	return proxyURL, nil
}

// parseProxyURL валидирует и парсит URL прокси
func parseProxyURL(raw string) (*url.URL, error) {
	parsedURL, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("некорректный proxy URL %q: требуется формат вида http://127.0.0.1:8080", raw)
	}
	return parsedURL, nil
}

// connectWebsocketAndHandshake устанавливает WebSocket соединение и выполняет handshake v3
// Возвращает websocket connection, команду сервера, параметры и ошибку
func connectWebsocketAndHandshake(cfg *Config) (*websocket.Conn, string, map[string]int, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.Verify},
		},
	}

	if cfg.Proxy != "" {
		ntlmssp.NewNegotiateMessage(cfg.ProxyAuth.Domain, "")
		negmsg, err := ntlmssp.NewNegotiateMessage(cfg.ProxyAuth.Domain, "")
		if err != nil {
			return nil, "", nil, fmt.Errorf("error getting domain negotiate message: %v", err)
		}

		var tsproxy func(*http.Request) (*url.URL, error)
		if cfg.Proxy == "." {
			tsproxy = http.ProxyFromEnvironment
		} else {
			parsedProxyURL, err := parseProxyURL(cfg.Proxy)
			if err != nil {
				return nil, "", nil, fmt.Errorf("ошибка парсинга proxy URL '%s': %v", cfg.Proxy, err)
			}
			tsproxy = http.ProxyURL(parsedProxyURL)
		}

		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !cfg.Verify},
				Proxy:           tsproxy,
				ProxyConnectHeader: http.Header{
					"Proxy-Authorization": []string{string(negmsg)},
				},
			},
		}

		req, err := http.NewRequest("GET", cfg.Connect, nil)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error creating http request to %s: %v", cfg.Connect, err)
		}
		req.Header.Set("User-Agent", cfg.UserAgent)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error making http request to %s: %v", cfg.Connect, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			log.Printf("No proxy auth required. Will make standard request")
		} else if resp.StatusCode == 407 {
			ntlmchall := resp.Header.Get("Proxy-Authenticate")
			log.Printf("Got following challenge: %s", ntlmchall)
			if strings.Contains(ntlmchall, "NTLM") {
				ntlmchall = ntlmchall[5:]
				challengeMessage, errb := base64.StdEncoding.DecodeString(ntlmchall)
				if errb != nil {
					return nil, "", nil, fmt.Errorf("error getting base64 decode of challenge: %v", errb)
				}
				authenticateMessage, erra := ntlmssp.ProcessChallenge(challengeMessage, cfg.ProxyAuth.Username, cfg.ProxyAuth.Password)
				if erra != nil {
					log.Printf("Error getting auth message for challenge: %v", erra)
				}
				authMessage := fmt.Sprintf("NTLM %s", base64.StdEncoding.EncodeToString(authenticateMessage))
				httpClient = &http.Client{
					Transport: &http.Transport{
						Proxy: tsproxy,
						ProxyConnectHeader: http.Header{
							"Proxy-Authorization": []string{string(authMessage)},
						},
					},
				}
			} else if strings.Contains(ntlmchall, "Basic") {
				authCombo := fmt.Sprintf("%s:%s", cfg.ProxyAuth.Username, cfg.ProxyAuth.Password)
				authMessage := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(authCombo)))
				httpClient = &http.Client{
					Transport: &http.Transport{
						Proxy: tsproxy,
						ProxyConnectHeader: http.Header{
							"Proxy-Authorization": []string{authMessage},
						},
					},
				}
			} else {
				return nil, "", nil, fmt.Errorf("unknown proxy challenge: %s", ntlmchall)
			}
		} else {
			log.Printf("Unknown http response code: %d", resp.StatusCode)
		}
	}

	// Добавляем X-Agent-ID для дедупликации на сервере
	currentAgentID := getAgentID(cfg)
	log.Printf("Using agent ID: %s", currentAgentID)

	// Формируем версию агента для заголовка
	agentVersion := fmt.Sprintf("v%d", common.ProtocolVersion)

	wconn, _, err := websocket.Dial(context.Background(), cfg.Connect, &websocket.DialOptions{
		HTTPClient: httpClient,
		HTTPHeader: http.Header{
			"User-Agent":             []string{cfg.UserAgent},
			"Accept-Language":        []string{cfg.Password},
			"Sec-WebSocket-Protocol": []string{"chat"},
			"X-Agent-ID":             []string{currentAgentID},
			"X-Agent-Version":        []string{agentVersion},
		},
		Subprotocols: []string{"chat"},
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("error connecting to WebSocket: %v", err)
	}

	// === Протокол v3: читаем команду от сервера ===
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msgType, data, err := wconn.Read(ctx)
	if err != nil {
		wconn.Close(websocket.StatusInternalError, "failed to read command")
		return nil, "", nil, fmt.Errorf("failed to read server command: %v", err)
	}

	if msgType != websocket.MessageText {
		wconn.Close(websocket.StatusInternalError, "unexpected message type")
		return nil, "", nil, fmt.Errorf("unexpected message type: %v", msgType)
	}

	response := strings.TrimSpace(string(data))
	log.Printf("Server response: %s", response)

	// Парсим команду
	if strings.HasPrefix(response, common.ErrPrefix) {
		wconn.Close(websocket.StatusInternalError, "server error")
		return nil, "", nil, fmt.Errorf("server error: %s", response)
	}

	if response == common.CmdTunnel {
		return wconn, "TUNNEL", nil, nil
	}

	if strings.HasPrefix(response, common.CmdSleep) {
		// Режим SLEEP: "CMD SLEEP <interval> <jitter>"
		parts := strings.Fields(response)
		if len(parts) < 4 {
			wconn.Close(websocket.StatusInternalError, "invalid sleep command")
			return nil, "", nil, fmt.Errorf("invalid SLEEP command format: %s", response)
		}

		interval, err1 := strconv.Atoi(parts[2])
		jitter, err2 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil {
			wconn.Close(websocket.StatusInternalError, "invalid sleep params")
			return nil, "", nil, fmt.Errorf("invalid SLEEP parameters: %s", response)
		}

		params := map[string]int{"interval": interval, "jitter": jitter}
		return wconn, "SLEEP", params, nil
	}

	wconn.Close(websocket.StatusInternalError, "unknown command")
	return nil, "", nil, fmt.Errorf("unknown server command: %s", response)
}

// runWebsocketTunnel запускает yamux сессию и SOCKS5 сервер через WebSocket (блокирующая функция)
func runWebsocketTunnel(wconn *websocket.Conn, cfg *Config) error {
	server, err := socks5.New(getSocksConfig(cfg))
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}

	nc_over_ws := websocket.NetConn(context.Background(), wconn, websocket.MessageBinary)

	session, err := yamux.Server(nc_over_ws, transport.GetYamuxConfig())
	if err != nil {
		return fmt.Errorf("failed to create yamux session: %w", err)
	}

	log.Println("WebSocket tunnel mode: accepting streams")

	for {
		stream, err := session.Accept()
		if err != nil {
			if session.IsClosed() {
				log.Println("Session closed, exiting accept loop")
				return fmt.Errorf("session closed")
			}
			log.Printf("Error accepting stream: %v", err)
			continue
		}

		log.Println("Accepted stream")
		go func() {
			err := server.ServeConn(stream)
			if err != nil {
				log.Printf("Error serving: %v", err)
			}
		}()
	}
}

// StartWebsocketBeaconLoop запускает основной цикл агента в beacon режиме через WebSocket
// Поддерживает как TUNNEL так и SLEEP режимы на основе команд сервера
func StartWebsocketBeaconLoop(cfg *Config) error {
	backoffInterval := 10 * time.Second // Базовый интервал при ошибке

	for {
		wconn, cmd, params, err := connectWebsocketAndHandshake(cfg)
		if err != nil {
			log.Printf("WebSocket handshake failed: %v", err)
			log.Printf("Sleeping %v before retry...", backoffInterval)
			time.Sleep(backoffInterval)
			continue
		}

		switch cmd {
		case "TUNNEL":
			log.Println("Server command: TUNNEL mode (WebSocket)")
			err := runWebsocketTunnel(wconn, cfg)
			if err != nil {
				log.Printf("WebSocket tunnel error: %v", err)
			}
			wconn.Close(websocket.StatusNormalClosure, "tunnel ended")
			// После разрыва туннеля пытаемся переподключиться с небольшой задержкой
			log.Println("Tunnel disconnected, reconnecting...")
			time.Sleep(5 * time.Second)

		case "SLEEP":
			interval := params["interval"]
			jitter := params["jitter"]
			sleepDuration := calculateJitter(interval, jitter)

			log.Printf("Server command: SLEEP %d sec (jitter %d%%) = ~%.1f sec",
				interval, jitter, sleepDuration.Seconds())

			wconn.Close(websocket.StatusNormalClosure, "sleep mode")
			time.Sleep(sleepDuration)
			log.Println("Waking up from sleep, checking in...")

		default:
			log.Printf("Unknown command: %s", cmd)
			if wconn != nil {
				wconn.Close(websocket.StatusInternalError, "unknown command")
			}
			time.Sleep(backoffInterval)
		}
	}
}

// ConnectWebsocket - deprecated, используйте StartWebsocketBeaconLoop
// Оставлено для обратной совместимости
func ConnectWebsocket(cfg *Config) error {
	return StartWebsocketBeaconLoop(cfg)
}

// ========================================
// Failover-совместимые функции (одна попытка)
// ========================================

// TryConnectWebsocket делает ОДНУ попытку подключения через WebSocket
// Возвращает соединение, команду сервера, параметры или ошибку
// Используется в failover режиме для контроля переключения серверов
func TryConnectWebsocket(cfg *Config) (*websocket.Conn, string, map[string]int, error) {
	return connectWebsocketAndHandshake(cfg)
}

// TryConnectTCP делает ОДНУ попытку подключения через TCP
// Возвращает соединение, команду сервера, параметры или ошибку
// Используется в failover режиме для контроля переключения серверов
func TryConnectTCP(cfg *Config) (net.Conn, string, map[string]int, error) {
	return connectAndHandshakeV3(cfg)
}

// RunWebsocketSession запускает сессию после успешного подключения через WebSocket
// Блокирует до разрыва сессии, затем возвращает ошибку
// cmd - команда от сервера (TUNNEL/SLEEP)
// params - параметры команды (interval, jitter для SLEEP)
func RunWebsocketSession(wconn *websocket.Conn, cfg *Config, cmd string, params map[string]int) error {
	switch cmd {
	case "TUNNEL":
		log.Println("Server command: TUNNEL mode (WebSocket)")
		err := runWebsocketTunnel(wconn, cfg)
		wconn.Close(websocket.StatusNormalClosure, "tunnel ended")
		return err

	case "SLEEP":
		interval := params["interval"]
		jitter := params["jitter"]
		sleepDuration := calculateJitter(interval, jitter)

		log.Printf("Server command: SLEEP %d sec (jitter %d%%) = ~%.1f sec",
			interval, jitter, sleepDuration.Seconds())

		wconn.Close(websocket.StatusNormalClosure, "sleep mode")
		time.Sleep(sleepDuration)
		log.Println("Waking up from sleep")
		return nil

	default:
		if wconn != nil {
			wconn.Close(websocket.StatusInternalError, "unknown command")
		}
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// RunTCPSession запускает сессию после успешного подключения через TCP
// Блокирует до разрыва сессии, затем возвращает ошибку
func RunTCPSession(conn net.Conn, cfg *Config, cmd string, params map[string]int) error {
	switch cmd {
	case "TUNNEL":
		log.Println("Server command: TUNNEL mode")
		return runTunnel(conn, cfg)

	case "SLEEP":
		interval := params["interval"]
		jitter := params["jitter"]
		sleepDuration := calculateJitter(interval, jitter)

		log.Printf("Server command: SLEEP %d sec (jitter %d%%) = ~%.1f sec",
			interval, jitter, sleepDuration.Seconds())

		conn.Close()
		time.Sleep(sleepDuration)
		log.Println("Waking up from sleep")
		return nil

	default:
		conn.Close()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// ========================================
// Beacon Loop (Handshake v3)
// ========================================

// calculateJitter вычисляет случайное время сна с учетом jitter
// base - базовый интервал в секундах
// jitterPercent - процент отклонения (например, 10 = ±10%)
func calculateJitter(baseSeconds int, jitterPercent int) time.Duration {
	if jitterPercent <= 0 {
		return time.Duration(baseSeconds) * time.Second
	}

	// Вычисляем delta
	delta := float64(baseSeconds) * (float64(jitterPercent) / 100.0)
	min := float64(baseSeconds) - delta
	max := float64(baseSeconds) + delta

	// Случайное значение в диапазоне [min, max]
	randomFloat := min + (max-min)*float64(common.RandBigInt(big.NewInt(10000)).Int64())/10000.0
	return time.Duration(randomFloat * float64(time.Second))
}

// connectAndHandshakeV3 подключается к серверу и выполняет handshake v3
// Возвращает conn, команду от сервера, параметры и ошибку
func connectAndHandshakeV3(cfg *Config) (net.Conn, string, map[string]int, error) {
	var conn net.Conn
	var err error

	conf := &tls.Config{
		InsecureSkipVerify: !cfg.Verify,
	}

	// Устанавливаем соединение
	if cfg.Proxy == "" {
		if cfg.UseTLS {
			conn, err = tls.Dial("tcp", cfg.Connect, conf)
		} else {
			conn, err = net.Dial("tcp", cfg.Connect)
		}
		if err != nil {
			return nil, "", nil, fmt.Errorf("connection failed: %w", err)
		}
	} else {
		connp := connectViaProxy(cfg, cfg.Connect)
		if connp == nil {
			return nil, "", nil, fmt.Errorf("proxy connection failed")
		}
		if cfg.UseTLS {
			conntls := tls.Client(connp, conf)
			err := conntls.Handshake()
			if err != nil {
				connp.Close()
				return nil, "", nil, fmt.Errorf("TLS handshake failed: %w", err)
			}
			conn = conntls
		} else {
			conn = connp
		}
	}

	// Получаем Agent ID
	currentAgentID := getAgentID(cfg)

	// Получаем текущие настройки yamux для передачи в handshake
	yamuxCfgStr := transport.GlobalYamuxSettings.EncodeHandshakeString()

	// Отправляем handshake v3: "AUTH <password> <agent_id> <version> <yamux_cfg>\n"
	handshakeMsg := fmt.Sprintf("AUTH %s %s v%d %s\n", cfg.Password, currentAgentID, common.ProtocolVersion, yamuxCfgStr)
	_, err = conn.Write([]byte(handshakeMsg))
	if err != nil {
		conn.Close()
		return nil, "", nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	// Читаем ответ сервера
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := reader.ReadString('\n')
	conn.SetReadDeadline(time.Time{}) // Сброс deadline
	if err != nil {
		conn.Close()
		return nil, "", nil, fmt.Errorf("failed to read server response: %w", err)
	}

	response = strings.TrimSpace(response)
	log.Printf("Server response: %s", response)

	// Парсим команду
	if strings.HasPrefix(response, common.ErrPrefix) {
		conn.Close()
		return nil, "", nil, fmt.Errorf("server error: %s", response)
	}

	if response == common.CmdTunnel {
		// Режим TUNNEL
		return conn, "TUNNEL", nil, nil
	}

	if strings.HasPrefix(response, common.CmdSleep) {
		// Режим SLEEP: "CMD SLEEP <interval> <jitter>"
		parts := strings.Fields(response)
		if len(parts) < 4 {
			conn.Close()
			return nil, "", nil, fmt.Errorf("invalid SLEEP command format: %s", response)
		}

		interval, err1 := strconv.Atoi(parts[2])
		jitter, err2 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil {
			conn.Close()
			return nil, "", nil, fmt.Errorf("invalid SLEEP parameters: %s", response)
		}

		params := map[string]int{"interval": interval, "jitter": jitter}
		return conn, "SLEEP", params, nil
	}

	conn.Close()
	return nil, "", nil, fmt.Errorf("unknown server command: %s", response)
}

// runTunnel запускает yamux сессию и SOCKS5 сервер (блокирующая функция)
func runTunnel(conn net.Conn, cfg *Config) error {
	server, err := socks5.New(getSocksConfig(cfg))
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 server: %w", err)
	}

	session, err := yamux.Server(conn, transport.GetYamuxConfig())
	if err != nil {
		return fmt.Errorf("failed to create yamux session: %w", err)
	}

	log.Println("Tunnel mode: accepting streams")

	for {
		stream, err := session.Accept()
		if err != nil {
			if session.IsClosed() {
				log.Println("Session closed, exiting tunnel")
				return fmt.Errorf("session closed")
			}
			log.Printf("Error accepting stream: %v", err)
			continue
		}

		log.Println("Accepted stream in tunnel mode")
		go func() {
			err := server.ServeConn(stream)
			if err != nil {
				log.Printf("Error serving stream: %v", err)
			}
		}()
	}
}

// StartBeaconLoop запускает основной цикл агента в beacon режиме
// Поддерживает как TUNNEL так и SLEEP режимы на основе команд сервера
func StartBeaconLoop(cfg *Config) error {
	backoffInterval := 10 * time.Second // Базовый интервал при ошибке

	for {
		conn, cmd, params, err := connectAndHandshakeV3(cfg)
		if err != nil {
			log.Printf("Handshake failed: %v", err)
			log.Printf("Sleeping %v before retry...", backoffInterval)
			time.Sleep(backoffInterval)
			continue
		}

		switch cmd {
		case "TUNNEL":
			log.Println("Server command: TUNNEL mode")
			err := runTunnel(conn, cfg)
			if err != nil {
				log.Printf("Tunnel error: %v", err)
			}
			// После разрыва туннеля пытаемся переподключиться с небольшой задержкой
			log.Println("Tunnel disconnected, reconnecting...")
			time.Sleep(5 * time.Second)

		case "SLEEP":
			interval := params["interval"]
			jitter := params["jitter"]
			sleepDuration := calculateJitter(interval, jitter)

			log.Printf("Server command: SLEEP %d sec (jitter %d%%) = ~%.1f sec",
				interval, jitter, sleepDuration.Seconds())

			conn.Close()
			time.Sleep(sleepDuration)
			log.Println("Waking up from sleep, checking in...")

		default:
			log.Printf("Unknown command: %s", cmd)
			time.Sleep(backoffInterval)
		}
	}
}

// connectViaProxy устанавливает соединение через прокси
func connectViaProxy(cfg *Config, connectaddr string) net.Conn {
	var proxyurl *url.URL
	connectproxystring := ""

	proxyurl = nil
	proxyaddr := cfg.Proxy

	if cfg.Proxy == "." {
		prefixstr := "https://"
		sysproxy, err := GetSystemProxy("POST", prefixstr+connectaddr)
		if err != nil {
			log.Printf("Error getting system proxy for %s: %v", prefixstr+connectaddr, err)
			return nil
		}
		proxyurl = sysproxy
		proxyaddr = sysproxy.Host
	}

	if cfg.ProxyAuth != nil && cfg.ProxyAuth.Username != "" && cfg.ProxyAuth.Password != "" && cfg.ProxyAuth.Domain != "" {
		negotiateMessage, errn := ntlmssp.NewNegotiateMessage(cfg.ProxyAuth.Domain, "")
		if errn != nil {
			log.Println("NEG error")
			log.Println(errn)
		}
		log.Print(negotiateMessage)
		negheader := fmt.Sprintf("NTLM %s", base64.StdEncoding.EncodeToString(negotiateMessage))

		connectproxystring = "CONNECT " + connectaddr + " HTTP/1.1" + "\r\nHost: " + connectaddr +
			"\r\nUser-Agent: " + cfg.UserAgent +
			"\r\nProxy-Authorization: " + negheader +
			"\r\nProxy-Connection: Keep-Alive" +
			"\r\n\r\n"
	} else {
		connectproxystring = "CONNECT " + connectaddr + " HTTP/1.1" + "\r\nHost: " + connectaddr +
			"\r\nUser-Agent: " + cfg.UserAgent +
			"\r\nProxy-Connection: Keep-Alive" +
			"\r\n\r\n"
	}

	if cfg.Debug {
		log.Print(sanitizeProxyConnect(connectproxystring))
	}

	conn, err := net.Dial("tcp", proxyaddr)
	if err != nil {
		log.Printf("Error connect to %s: %v", proxyaddr, err)
		return nil
	}
	conn.Write([]byte(connectproxystring))

	time.Sleep(cfg.ProxyTimeout)

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil || resp == nil {
		log.Printf("Error reading proxy response: %v", err)
		if conn != nil {
			conn.Close()
		}
		return nil
	}
	status := resp.Status

	if cfg.Debug {
		log.Print(status)
		log.Print(resp)
	}

	if (resp.StatusCode == 200) || (strings.Contains(status, "HTTP/1.1 200 ")) ||
		(strings.Contains(status, "HTTP/1.0 200 ")) {
		log.Print("Connected via proxy. No auth required")
		return conn
	}

	if cfg.Debug {
		log.Print("Checking proxy auth")
	}

	if resp.StatusCode == 407 {
		log.Print("Got Proxy status code (407)")
		ntlmchall := resp.Header.Get("Proxy-Authenticate")
		log.Print(ntlmchall)

		if strings.Contains(ntlmchall, "NTLM") {
			if cfg.Debug {
				log.Print("Got NTLM challenge:")
				log.Print(ntlmchall)
			}

			ntlmchall = ntlmchall[5:]
			if cfg.Debug {
				log.Print("NTLM challenge:")
				log.Print(ntlmchall)
			}
			challengeMessage, errb := decBase64(ntlmchall)
			if errb != nil {
				log.Println("BASE64 Decode error")
				log.Println(errb)
				return nil
			}

			user := ""
			pass := ""
			if cfg.Proxy == "." && proxyurl != nil {
				user = proxyurl.User.Username()
				p, pset := proxyurl.User.Password()
				if pset {
					pass = p
				}
			}
			if cfg.ProxyAuth != nil && cfg.ProxyAuth.Username != "" && cfg.ProxyAuth.Password != "" {
				user = cfg.ProxyAuth.Username
				pass = cfg.ProxyAuth.Password
			}

			authenticateMessage, erra := ntlmssp.ProcessChallenge(challengeMessage, user, pass)
			if erra != nil {
				log.Println("Process challenge error")
				log.Println(erra)
				return nil
			}

			authMessage := fmt.Sprintf("NTLM %s", base64.StdEncoding.EncodeToString(authenticateMessage))

			connectproxystring = "CONNECT " + connectaddr + " HTTP/1.1" + "\r\nHost: " + connectaddr +
				"\r\nUser-Agent: Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko" +
				"\r\nProxy-Authorization: " + authMessage +
				"\r\nProxy-Connection: Keep-Alive" +
				"\r\n\r\n"
		} else if strings.Contains(ntlmchall, "Basic") {
			if cfg.Debug {
				log.Print("Got Basic challenge:")
			}
			var authbuffer bytes.Buffer
			if cfg.ProxyAuth != nil && cfg.ProxyAuth.Username != "" && cfg.ProxyAuth.Password != "" {
				authbuffer.WriteString(cfg.ProxyAuth.Username)
				authbuffer.WriteString(":")
				authbuffer.WriteString(cfg.ProxyAuth.Password)
			} else if cfg.Proxy == "." && proxyurl != nil {
				authbuffer.WriteString(proxyurl.User.String())
			}

			basicauth := encBase64(authbuffer.Bytes())

			connectproxystring = "CONNECT " + connectaddr + " HTTP/1.1" + "\r\nHost: " + connectaddr +
				"\r\nUser-Agent: Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko" +
				"\r\nProxy-Authorization: Basic " + basicauth +
				"\r\nProxy-Connection: Keep-Alive" +
				"\r\n\r\n"
		} else {
			log.Print("Unknown authentication")
			return nil
		}

		log.Print("Connecting to proxy")
		log.Print(sanitizeProxyConnect(connectproxystring))
		conn.Write([]byte(connectproxystring))

		bufReader := bufio.NewReader(conn)
		conn.SetReadDeadline(time.Now().Add(cfg.ProxyTimeout))
		statusb, _ := io.ReadAll(bufReader)
		status = string(statusb)

		conn.SetReadDeadline(time.Now().Add(100 * time.Hour))

		if resp.StatusCode == 200 || strings.Contains(status, "HTTP/1.1 200 ") ||
			strings.Contains(status, "HTTP/1.0 200 ") {
			log.Print("Connected via proxy")
			return conn
		}
		log.Printf("Not Connected via proxy. Status:%v", status)
		return nil
	}

	log.Print("Not connected via proxy")
	conn.Close()
	return nil
}

