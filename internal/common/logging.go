package common

import "log"

// DebugMode - глобальный флаг для включения подробного логирования
// В production (false) - только важные события: подключения агентов, ошибки
// В debug (true) - всё: HTTP запросы, редиректы, копирование стримов
var DebugMode = false

// SetDebugMode устанавливает режим отладки
func SetDebugMode(enabled bool) {
	DebugMode = enabled
	if enabled {
		log.Println("[DEBUG] Debug logging enabled - verbose output active")
	}
}

// DebugLog логирует сообщение только в debug режиме
// Используется для "шумных" операций: HTTP запросы, редиректы, stream I/O
func DebugLog(format string, v ...interface{}) {
	if DebugMode {
		log.Printf(format, v...)
	}
}
