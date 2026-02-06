package e2e

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/net/proxy"
)

// TestProxyConnection проверяет работу SOCKS5 прокси
// Подключается к targetAddr через proxyAddr и отправляет testData
// Ожидает echo ответа и сравнивает данные
func TestProxyConnection(proxyAddr, targetAddr string, testData []byte) error {
	// Создаём SOCKS5 диалер
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Подключаемся к target через прокси
	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial %s through proxy %s: %w", targetAddr, proxyAddr, err)
	}
	defer conn.Close()

	// Отправляем тестовые данные
	if _, err := conn.Write(testData); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Читаем echo ответ
	buf := make([]byte, len(testData))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	// Проверяем что данные совпадают
	if !bytes.Equal(buf, testData) {
		return fmt.Errorf("data mismatch: got %v, want %v", buf, testData)
	}

	return nil
}

// TestProxyConnectionWithAuth проверяет работу SOCKS5 прокси с аутентификацией.
// Важно: аутентификация применяется на стороне агента (SOCKS-сервер), а сервер RevSocks лишь проксирует TCP к агенту.
func TestProxyConnectionWithAuth(proxyAddr, targetAddr string, testData []byte, user, pass string) error {
	// Создаём SOCKS5 диалер с RFC1929 auth
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, &proxy.Auth{
		User:     user,
		Password: pass,
	}, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer with auth: %w", err)
	}

	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial %s through proxy %s (auth): %w", targetAddr, proxyAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write(testData); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	buf := make([]byte, len(testData))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	if !bytes.Equal(buf, testData) {
		return fmt.Errorf("data mismatch: got %v, want %v", buf, testData)
	}

	return nil
}
