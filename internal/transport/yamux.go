package transport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
)

// ========================================
// Yamux Configuration
// ========================================

// YamuxSettings хранит настройки yamux
// Может быть изменена через CLI флаги или build_stealth.sh
type YamuxSettings struct {
	KeepAliveInterval time.Duration
	WriteTimeout      time.Duration
	EnableKeepAlive   bool
}

// DefaultYamuxSettings возвращает дефолтные настройки yamux
func DefaultYamuxSettings() *YamuxSettings {
	return &YamuxSettings{
		KeepAliveInterval: 30 * time.Second,
		WriteTimeout:      10 * time.Second,
		EnableKeepAlive:   true,
	}
}

// NewYamuxConfig создаёт yamux конфигурацию с указанными настройками
// Используется и сервером и клиентом для согласованных таймаутов
func NewYamuxConfig(settings *YamuxSettings) *yamux.Config {
	if settings == nil {
		settings = DefaultYamuxSettings()
	}
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = settings.EnableKeepAlive
	config.KeepAliveInterval = settings.KeepAliveInterval
	config.ConnectionWriteTimeout = settings.WriteTimeout
	return config
}

// UpdateSettings обновляет настройки из CLI флагов
func (s *YamuxSettings) UpdateSettings(keepaliveSeconds, timeoutSeconds int) {
	if keepaliveSeconds > 0 {
		s.KeepAliveInterval = time.Duration(keepaliveSeconds) * time.Second
	}
	if timeoutSeconds > 0 {
		s.WriteTimeout = time.Duration(timeoutSeconds) * time.Second
	}
}

// GlobalYamuxSettings - глобальные настройки yamux
// Инициализируется с дефолтами, обновляется после парсинга флагов
var GlobalYamuxSettings = DefaultYamuxSettings()

// GetYamuxConfig возвращает yamux конфигурацию с текущими глобальными настройками
func GetYamuxConfig() *yamux.Config {
	return NewYamuxConfig(GlobalYamuxSettings)
}

// ========================================
// Handshake v3: Yamux Settings Negotiation
// ========================================

// EncodeHandshakeString кодирует настройки yamux для передачи в handshake v3
// Формат: "yamux:<keepalive_sec>:<timeout_sec>:<enabled>"
// Пример: "yamux:30:10:1"
func (s *YamuxSettings) EncodeHandshakeString() string {
	enabled := 0
	if s.EnableKeepAlive {
		enabled = 1
	}
	return fmt.Sprintf("yamux:%d:%d:%d",
		int(s.KeepAliveInterval.Seconds()),
		int(s.WriteTimeout.Seconds()),
		enabled,
	)
}

// ParseYamuxHandshake парсит строку настроек yamux из handshake
// Возвращает настройки или ошибку
func ParseYamuxHandshake(encoded string) (*YamuxSettings, error) {
	if !strings.HasPrefix(encoded, "yamux:") {
		return nil, fmt.Errorf("invalid yamux config format: must start with 'yamux:'")
	}

	parts := strings.Split(encoded, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid yamux config format: expected 4 parts, got %d", len(parts))
	}

	keepalive, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid keepalive value: %w", err)
	}

	timeout, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid timeout value: %w", err)
	}

	enabled, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid enabled value: %w", err)
	}

	return &YamuxSettings{
		KeepAliveInterval: time.Duration(keepalive) * time.Second,
		WriteTimeout:      time.Duration(timeout) * time.Second,
		EnableKeepAlive:   enabled == 1,
	}, nil
}

// ValidateClientSettings проверяет совпадение настроек клиента с сервером
// Возвращает nil если настройки совпадают, иначе - ошибку с деталями
func ValidateClientSettings(clientSettings *YamuxSettings, serverSettings *YamuxSettings) error {
	if serverSettings == nil {
		serverSettings = GlobalYamuxSettings
	}

	var mismatches []string

	if clientSettings.KeepAliveInterval != serverSettings.KeepAliveInterval {
		mismatches = append(mismatches, fmt.Sprintf("KeepAlive: client=%ds server=%ds",
			int(clientSettings.KeepAliveInterval.Seconds()),
			int(serverSettings.KeepAliveInterval.Seconds())))
	}

	if clientSettings.WriteTimeout != serverSettings.WriteTimeout {
		mismatches = append(mismatches, fmt.Sprintf("WriteTimeout: client=%ds server=%ds",
			int(clientSettings.WriteTimeout.Seconds()),
			int(serverSettings.WriteTimeout.Seconds())))
	}

	if clientSettings.EnableKeepAlive != serverSettings.EnableKeepAlive {
		mismatches = append(mismatches, fmt.Sprintf("EnableKeepAlive: client=%t server=%t",
			clientSettings.EnableKeepAlive, serverSettings.EnableKeepAlive))
	}

	if len(mismatches) > 0 {
		return fmt.Errorf("yamux config mismatch: %s", strings.Join(mismatches, ", "))
	}

	return nil
}
