package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kost/revsocks/internal/common"
	"github.com/kost/revsocks/internal/dns"
	"github.com/kost/revsocks/internal/server"
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
	listen         string
	certificate    string
	socks          string
	password       string
	autocert       string
	proxytimeout   string
	usetls         bool
	usewebsocket   bool
	debug          bool
	quiet          bool
	yamuxKeepalive int
	yamuxTimeout   int
	// DNS mode
	dnslisten string
	dnsdomain string
	dnsdelay  string
	// Agent management
	agentdb string // Путь к БД агентов (JSON файл)
	// Admin API
	adminAPI  bool   // Включить Admin API
	adminPort string // Порт для Admin API (только localhost)
}

func main() {
	var opts AppOptions

	// Основные флаги
	flag.StringVar(&opts.listen, "listen", "", "listen port for agents address:port")
	flag.StringVar(&opts.certificate, "cert", "", "certificate file prefix (without .crt/.key)")
	flag.StringVar(&opts.socks, "socks", "127.0.0.1:1080", "socks listen address:port for clients")
	flag.StringVar(&opts.password, "pass", "", "Connect password")
	flag.StringVar(&opts.autocert, "autocert", "", "use domain.tld for automatic TLS certificate")
	flag.StringVar(&opts.proxytimeout, "proxytimeout", "", "proxy response timeout (ms)")

	// TLS
	flag.BoolVar(&opts.usetls, "tls", false, "use TLS for connection")
	flag.BoolVar(&opts.usewebsocket, "ws", false, "use websocket for connection")

	// Yamux tuning
	flag.IntVar(&opts.yamuxKeepalive, "yamux-keepalive", 30, "yamux keepalive interval in seconds")
	flag.IntVar(&opts.yamuxTimeout, "yamux-timeout", 10, "yamux write timeout in seconds")

	// DNS mode
	flag.StringVar(&opts.dnslisten, "dnslisten", "", "Where should DNS server listen")
	flag.StringVar(&opts.dnsdomain, "dns", "", "DNS domain to use for DNS tunneling")
	flag.StringVar(&opts.dnsdelay, "dnsdelay", "", "Delay/sleep time between DNS requests")

	// Agent management
	flag.StringVar(&opts.agentdb, "agentdb", "./agents.json", "Path to agents database (JSON file)")

	// Admin API (только localhost, без авторизации)
	flag.BoolVar(&opts.adminAPI, "admin-api", false, "Enable Admin HTTP API (localhost only)")
	flag.StringVar(&opts.adminPort, "admin-port", "127.0.0.1:8081", "Admin API listen address:port")

	// Misc
	flag.BoolVar(&opts.debug, "debug", false, "display debug info")
	flag.BoolVar(&opts.quiet, "q", false, "Be quiet - do not display output")
	version := flag.Bool("version", false, "version information")

	flag.Usage = func() {
		fmt.Printf("revsocks-server - reverse socks5 server %s (%s)\n", common.Version, common.CommitID)
		fmt.Println("")
		flag.PrintDefaults()
		fmt.Println("")
		fmt.Println("Usage (standard tcp):")
		fmt.Println("  revsocks-server -listen :8080 -socks 127.0.0.1:1080 -pass test -tls")
		fmt.Println("")
		fmt.Println("Usage (dns tunneling):")
		fmt.Println("  revsocks-server -dns example.com -dnslisten :53 -socks 127.0.0.1:1080")
	}

	flag.Parse()

	// Обновляем yamux конфигурацию
	transport.GlobalYamuxSettings.UpdateSettings(opts.yamuxKeepalive, opts.yamuxTimeout)

	// Инициализируем signal handler
	setupSignalHandler()

	if opts.quiet {
		log.SetOutput(io.Discard)
	}

	// Включаем debug режим если указан флаг
	if opts.debug {
		common.SetDebugMode(true)
	}

	if *version {
		fmt.Printf("revsocks-server - reverse socks5 server %s (%s)\n", common.Version, common.CommitID)
		os.Exit(0)
	}

	// Генерируем пароль если не указан
	if opts.password == "" {
		opts.password = common.RandString(64)
		log.Printf("No password specified. Generated password is %s", opts.password)
	}

	// Парсим proxy timeout
	proxyTimeout := time.Millisecond * 1000
	if opts.proxytimeout != "" {
		ms, err := strconv.Atoi(opts.proxytimeout)
		if err != nil {
			log.Fatalf("Invalid proxytimeout value '%s': must be integer (milliseconds)", opts.proxytimeout)
		}
		if ms <= 0 {
			log.Fatalf("Invalid proxytimeout value: must be positive integer")
		}
		proxyTimeout = time.Millisecond * time.Duration(ms)
	}

	// ========================================
	// DNS Server Mode
	// ========================================
	if opts.dnsdomain != "" && opts.dnslisten != "" {
		dnskey := opts.password
		if opts.password == "" {
			dnskey = dns.GenerateKey()
			log.Printf("No password specified, generated following (recheck if same on both sides): %s", dnskey)
		}
		if len(dnskey) != 64 {
			fmt.Fprintf(os.Stderr, "Specified key of incorrect size for DNS (should be 64 in hex)\n")
			os.Exit(1)
		}

		cfg := &dns.ServerConfig{
			DNSListen:     opts.dnslisten,
			DNSDomain:     opts.dnsdomain,
			ClientsListen: opts.socks,
			EncryptionKey: dnskey,
			DNSDelay:      opts.dnsdelay,
		}
		log.Fatal(dns.ServeDNS(cfg))
	}

	// ========================================
	// TCP/WebSocket Server Mode
	// ========================================
	if opts.listen == "" {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "You must specify a listen port\n")
		os.Exit(1)
	}

	// Инициализируем AgentManager
	agentManager, err := server.NewAgentManager(opts.agentdb)
	if err != nil {
		log.Fatalf("Failed to initialize AgentManager: %v", err)
	}
	log.Printf("AgentManager initialized with database: %s", opts.agentdb)

	// Запускаем Admin API если включён (localhost only, без авторизации)
	if opts.adminAPI {
		apiCfg := &server.AdminAPIConfig{
			ListenAddr:     opts.adminPort,
			AgentManager:   agentManager,
			SessionManager: server.GlobalSessionManager,
		}

		// Запускаем API в отдельной горутине
		go func() {
			if err := server.StartAdminServer(apiCfg); err != nil {
				log.Fatalf("Failed to start Admin API: %v", err)
			}
		}()
	}

	cfg := &server.Config{
		ListenAddress:  opts.listen,
		ClientsListen:  opts.socks,
		UseTLS:         opts.usetls,
		Certificate:    opts.certificate,
		AutocertDomain: opts.autocert,
		Password:       opts.password,
		ProxyTimeout:   proxyTimeout,
		AgentManager:   agentManager,
	}

	log.Println("Starting to listen for agents")

	if opts.usewebsocket {
		log.Fatal(server.ListenWebsocket(cfg))
	} else {
		log.Fatal(server.Listen(cfg))
	}
}
