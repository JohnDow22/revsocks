package e2e

import (
	"io"
	"net"
	"sync"
)

// EchoServer - простой TCP echo сервер для тестирования проксирования
type EchoServer struct {
	listener net.Listener
	Addr     string // Реальный адрес после Listen (например "127.0.0.1:34567")

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewEchoServer создаёт и запускает echo сервер на случайном порту
func NewEchoServer() (*EchoServer, error) {
	// Слушаем на случайном порту (OS выберет свободный)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	es := &EchoServer{
		listener: ln,
		Addr:     ln.Addr().String(),
		stopChan: make(chan struct{}),
	}

	// Запускаем accept loop в горутине
	es.wg.Add(1)
	go es.serve()

	return es, nil
}

// serve принимает входящие подключения и обрабатывает их
func (es *EchoServer) serve() {
	defer es.wg.Done()

	for {
		// Проверяем сигнал остановки
		select {
		case <-es.stopChan:
			return
		default:
		}

		// Устанавливаем небольшой таймаут чтобы не висеть в Accept навсегда
		conn, err := es.listener.Accept()
		if err != nil {
			// Listener закрыт или ошибка - выходим
			select {
			case <-es.stopChan:
				return
			default:
				// Продолжаем если это временная ошибка
				continue
			}
		}

		// Обрабатываем соединение в отдельной горутине
		es.wg.Add(1)
		go es.handleConn(conn)
	}
}

// handleConn читает данные и отправляет их обратно (echo)
func (es *EchoServer) handleConn(conn net.Conn) {
	defer es.wg.Done()
	defer conn.Close()

	// Копируем всё что пришло обратно клиенту
	io.Copy(conn, conn)
}

// Close останавливает сервер
func (es *EchoServer) Close() error {
	close(es.stopChan)
	
	// Закрываем listener (это разблокирует Accept)
	if err := es.listener.Close(); err != nil {
		return err
	}

	// Ждём завершения всех горутин
	es.wg.Wait()

	return nil
}
