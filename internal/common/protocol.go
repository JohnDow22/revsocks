package common

// Константы протокола handshake
// Общие для сервера и клиента
const (
	// Версия протокола handshake
	ProtocolVersion  = 3
	PasswordSize     = 64
	MaxAgentIDLength = 255

	// Handshake v2 responses (deprecated для v3, но оставлены для совместимости)
	HandshakeACK  = "OK"
	HandshakeNACK = "NO"

	// Handshake v3 commands (text-based protocol)
	CmdTunnel = "CMD TUNNEL"    // Серверная команда: начать tunnel режим
	CmdSleep  = "CMD SLEEP"     // Серверная команда: спать с параметрами
	ErrPrefix = "ERR "          // Префикс ошибки от сервера
	AuthOK    = "AUTH OK"       // Успешная аутентификация
	AuthFail  = "ERR Auth Failed" // Ошибка аутентификации

	// Yamux configuration mismatch errors
	ErrYamuxMismatch = "ERR Yamux Config Mismatch" // Ошибка: настройки yamux не совпадают
)
