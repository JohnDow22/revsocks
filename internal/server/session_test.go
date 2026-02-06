package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/kost/revsocks/internal/transport"
)

// newYamuxPair создаёт пару yamux-сессий на net.Pipe().
// Это даёт "настоящий" *yamux.Session для тестов lifecycle без моков.
func newYamuxPair(t *testing.T) (client *yamux.Session, server *yamux.Session, cleanup func()) {
	t.Helper()

	c1, c2 := net.Pipe()

	clientSess, err := yamux.Client(c1, transport.GetYamuxConfig())
	if err != nil {
		_ = c1.Close()
		_ = c2.Close()
		t.Fatalf("Не удалось создать yamux.Client: %v", err)
	}

	serverSess, err := yamux.Server(c2, transport.GetYamuxConfig())
	if err != nil {
		_ = clientSess.Close()
		_ = c1.Close()
		_ = c2.Close()
		t.Fatalf("Не удалось создать yamux.Server: %v", err)
	}

	return clientSess, serverSess, func() {
		_ = clientSess.Close()
		_ = serverSess.Close()
		_ = c1.Close()
		_ = c2.Close()
	}
}

// dialMustFail помогает проверить, что listener действительно закрыт.
func dialMustFail(t *testing.T, addr string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("Ожидали ошибку подключения к %s (listener должен быть закрыт), но Dial прошёл", addr)
	}
}

func TestSessionManager_RegisterSession_ReusesPortCache(t *testing.T) {
	sm := NewSessionManager()

	_, sess1, cleanup1 := newYamuxPair(t)
	defer cleanup1()

	_, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	gen1, port1 := sm.RegisterSession("agent-1", sess1, 50001, cancel1)
	if gen1 == 0 {
		t.Fatalf("generation должен быть > 0")
	}
	if port1 != 50001 {
		t.Fatalf("ожидали port=50001, получили %d", port1)
	}

	if err := sm.CloseSession("agent-1"); err != nil {
		t.Fatalf("CloseSession вернул ошибку: %v", err)
	}

	_, sess2, cleanup2 := newYamuxPair(t)
	defer cleanup2()

	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	_, port2 := sm.RegisterSession("agent-1", sess2, 60001, cancel2)
	// Контракт: portCache сохраняется между сессиями агента.
	if port2 != port1 {
		t.Fatalf("ожидали переиспользование порта %d из кэша, получили %d", port1, port2)
	}
}

func TestSessionManager_SetListener_WrongGenerationIgnored(t *testing.T) {
	sm := NewSessionManager()

	_, sess, cleanup := newYamuxPair(t)
	defer cleanup()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	gen, _ := sm.RegisterSession("agent-1", sess, 50001, cancel)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Не удалось открыть listener: %v", err)
	}
	defer ln.Close()

	// Ошибочный generation не должен привязать listener к активной сессии.
	if ok := sm.SetListener("agent-1", gen+999, ln); ok {
		t.Fatalf("SetListener должен вернуть false при неверном generation")
	}
	if got := sm.GetSocksAddr("agent-1"); got != "" {
		t.Fatalf("ожидали пустой socks addr (listener не должен быть установлен), получили %q", got)
	}
}

func TestSessionManager_UnregisterSession_GenerationMismatchDoesNotDeleteNewSession(t *testing.T) {
	sm := NewSessionManager()

	// Первая сессия.
	_, sess1, cleanup1 := newYamuxPair(t)
	defer cleanup1()

	_, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	gen1, _ := sm.RegisterSession("agent-1", sess1, 50001, cancel1)

	// Вторая сессия (reconnect), должна заменить первую.
	_, sess2, cleanup2 := newYamuxPair(t)
	defer cleanup2()

	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	gen2, _ := sm.RegisterSession("agent-1", sess2, 50001, cancel2)
	if gen2 == gen1 {
		t.Fatalf("generation должен меняться при новой сессии")
	}

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Не удалось открыть listener: %v", err)
	}
	defer ln2.Close()

	if ok := sm.SetListener("agent-1", gen2, ln2); !ok {
		t.Fatalf("SetListener должен вернуть true для актуальной сессии")
	}

	// Пытаемся удалить "старую" сессию по gen1: это не должно затронуть новую (gen2).
	sm.UnregisterSession("agent-1", gen1)

	if got := sm.GetSocksAddr("agent-1"); got == "" {
		t.Fatalf("ожидали, что актуальная сессия останется зарегистрированной (socks addr не пустой)")
	}

	// Если UnregisterSession по ошибке закроет ln2 — Dial должен упасть.
	// Если всё ок, Dial может пройти (и его нужно закрыть).
	conn, err := net.DialTimeout("tcp", ln2.Addr().String(), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("ожидали, что listener всё ещё открыт, но Dial упал: %v", err)
	}
	_ = conn.Close()
}

func TestSessionManager_CloseSession_ClosesListenerAndRemovesSession(t *testing.T) {
	sm := NewSessionManager()

	_, sess, cleanup := newYamuxPair(t)
	defer cleanup()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	gen, _ := sm.RegisterSession("agent-1", sess, 50001, cancel)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Не удалось открыть listener: %v", err)
	}
	addr := ln.Addr().String()

	if ok := sm.SetListener("agent-1", gen, ln); !ok {
		_ = ln.Close()
		t.Fatalf("SetListener должен вернуть true для актуальной сессии")
	}

	if err := sm.CloseSession("agent-1"); err != nil {
		t.Fatalf("CloseSession вернул ошибку: %v", err)
	}

	if sm.GetSessionCount() != 0 {
		t.Fatalf("ожидали 0 активных сессий после CloseSession, получили %d", sm.GetSessionCount())
	}

	// Проверяем, что listener закрыт (должен отказать в подключении).
	dialMustFail(t, addr)
}

func TestSessionManager_CloseSession_NotFound(t *testing.T) {
	sm := NewSessionManager()
	if err := sm.CloseSession("missing-agent"); err == nil {
		t.Fatalf("ожидали ошибку для несуществующей сессии")
	}
}

