package dns

import (
	"log"
	"time"

	socks5 "github.com/armon/go-socks5"
	"github.com/kost/dnstun"
)

// GenerateKey генерирует ключ для DNS туннеля
func GenerateKey() string {
	return dnstun.GenerateKey()
}

// ClientConfig содержит настройки для DNS клиента
type ClientConfig struct {
	TargetDomain  string
	EncryptionKey string
	DNSDelay      string
	SocksConfig   *socks5.Config
}

// ConnectSocks устанавливает DNS-туннель и проксирует SOCKS5 запросы
func ConnectSocks(cfg *ClientConfig) error {
	server, err := socks5.New(cfg.SocksConfig)
	if err != nil {
		log.Printf("Error socks5.new:  %v", err)
		return err
	}
	dt := dnstun.NewDnsTunnel(cfg.TargetDomain, cfg.EncryptionKey)
	if cfg.DNSDelay != "" {
		err = dt.SetDnsDelay(cfg.DNSDelay)
		if err != nil {
			log.Printf("Error setting delay:  %v", err)
			return err
		}
	}
	for {
		session, err := dt.DnsClient()
		if err != nil {
			log.Printf("Error yamux transport:  %v", err)
			return err
		}
		for {
			// Проверяем состояние сессии перед Accept
			if session.IsClosed() {
				log.Println("DNS session closed, reconnecting...")
				break
			}

			stream, err := session.Accept()
			log.Println("Accepting stream")
			if err != nil {
				// Проверяем снова - сессия могла закрыться во время Accept
				if session.IsClosed() {
					log.Println("DNS session closed during accept")
					break
				}
				log.Printf("Error accepting stream:  %v", err)
				continue // Не break - пробуем снова если сессия жива
			}
			log.Println("Passing off to socks5")
			go func() {
				err = server.ServeConn(stream)
				if err != nil {
					log.Println(err)
				}
			}()
		}
		// Добавляем backoff перед reconnect
		log.Println("DNS session ended, waiting 5 sec before reconnect...")
		time.Sleep(5 * time.Second)
	}
}

// ServerConfig содержит настройки для DNS сервера
type ServerConfig struct {
	DNSListen     string
	DNSDomain     string
	ClientsListen string
	EncryptionKey string
	DNSDelay      string
}

// ServeDNS запускает DNS сервер для туннелирования
func ServeDNS(cfg *ServerConfig) error {
	dt := dnstun.NewDnsTunnel(cfg.DNSDomain, cfg.EncryptionKey)
	if cfg.DNSDelay != "" {
		err := dt.SetDnsDelay(cfg.DNSDelay)
		if err != nil {
			log.Printf("Error parsing DNS delay/sleep duration %s: %v", cfg.DNSDelay, err)
			return err
		}
	}
	dt.DnsServer(cfg.DNSListen, cfg.ClientsListen)
	err := dt.DnsServerStart()
	if err != nil {
		log.Printf("Error starting DNS server %s: %v", cfg.DNSDomain, err)
		return err
	}
	return nil
}
