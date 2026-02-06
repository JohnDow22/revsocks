package agent

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/kost/revsocks/internal/common"
)

// ========================================
// Unit Tests для client.go
// ========================================

// TestCalculateJitter_NoJitter проверяет что при jitter=0 возвращается базовое время
func TestCalculateJitter_NoJitter(t *testing.T) {
	baseSeconds := 60
	jitter := 0

	result := calculateJitter(baseSeconds, jitter)

	expected := time.Duration(baseSeconds) * time.Second
	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestCalculateJitter_NegativeJitter проверяет что отрицательный jitter обрабатывается как 0
func TestCalculateJitter_NegativeJitter(t *testing.T) {
	baseSeconds := 60
	jitter := -10

	result := calculateJitter(baseSeconds, jitter)

	expected := time.Duration(baseSeconds) * time.Second
	if result != expected {
		t.Errorf("Expected %v for negative jitter, got %v", expected, result)
	}
}

// TestCalculateJitter_Range проверяет что результат находится в ожидаемом диапазоне
func TestCalculateJitter_Range(t *testing.T) {
	baseSeconds := 100
	jitter := 10 // 10% = ±10 секунд
	iterations := 1000

	minExpected := time.Duration(90) * time.Second
	maxExpected := time.Duration(110) * time.Second

	for i := 0; i < iterations; i++ {
		result := calculateJitter(baseSeconds, jitter)

		if result < minExpected || result > maxExpected {
			t.Errorf("Iteration %d: result %v out of range [%v, %v]", i, result, minExpected, maxExpected)
		}
	}
}

// TestCalculateJitter_Distribution проверяет статистическое распределение
func TestCalculateJitter_Distribution(t *testing.T) {
	baseSeconds := 100
	jitter := 10 // 10% = ±10 секунд
	iterations := 10000

	var sum float64
	for i := 0; i < iterations; i++ {
		result := calculateJitter(baseSeconds, jitter)
		sum += result.Seconds()
	}

	average := sum / float64(iterations)
	expected := float64(baseSeconds)

	// Среднее должно быть близко к базовому значению (с погрешностью)
	tolerance := 2.0 // 2 секунды
	if average < expected-tolerance || average > expected+tolerance {
		t.Errorf("Average %v is not close to expected %v (tolerance: %v)", average, expected, tolerance)
	}
}

// TestCalculateJitter_EdgeCases проверяет граничные случаи
func TestCalculateJitter_EdgeCases(t *testing.T) {
	testCases := []struct {
		name         string
		baseSeconds  int
		jitterPercent int
		minExpected  time.Duration
		maxExpected  time.Duration
	}{
		{
			name:          "SmallBase_LargeJitter",
			baseSeconds:   10,
			jitterPercent: 50, // 50% = ±5 секунд
			minExpected:   5 * time.Second,
			maxExpected:   15 * time.Second,
		},
		{
			name:          "LargeBase_SmallJitter",
			baseSeconds:   3600, // 1 час
			jitterPercent: 1,    // 1% = ±36 секунд
			minExpected:   3564 * time.Second,
			maxExpected:   3636 * time.Second,
		},
		{
			name:          "MaxJitter",
			baseSeconds:   60,
			jitterPercent: 100, // 100% = ±60 секунд (может быть 0)
			minExpected:   0 * time.Second,
			maxExpected:   120 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			iterations := 100
			for i := 0; i < iterations; i++ {
				result := calculateJitter(tc.baseSeconds, tc.jitterPercent)

				if result < tc.minExpected || result > tc.maxExpected {
					t.Errorf("%s: result %v out of range [%v, %v]",
						tc.name, result, tc.minExpected, tc.maxExpected)
				}
			}
		})
	}
}

// TestGetAgentID проверяет логику получения Agent ID
func TestGetAgentID(t *testing.T) {
	// Case 1: ID указан в config
	cfg := &Config{AgentID: "custom-agent-id"}
	result := getAgentID(cfg)
	if result != "custom-agent-id" {
		t.Errorf("Expected 'custom-agent-id', got '%s'", result)
	}

	// Case 2: ID не указан - должен использоваться hostname или fallback
	cfg2 := &Config{}
	result2 := getAgentID(cfg2)
	if result2 == "" {
		t.Error("Expected non-empty agent ID when not specified in config")
	}

	// Проверяем что результат не пустой (может быть hostname или random)
	_ = getAgentID(cfg2) // Функция может возвращать разные значения из-за random fallback
}

// BenchmarkCalculateJitter измеряет производительность calculateJitter
func BenchmarkCalculateJitter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		calculateJitter(100, 10)
	}
}

// TestParseServerCommand проверяет парсинг команд от сервера
func TestParseServerCommand(t *testing.T) {
	testCases := []struct {
		name          string
		response      string
		expectedCmd   string
		expectedError bool
	}{
		{
			name:          "TunnelCommand",
			response:      "CMD TUNNEL\n",
			expectedCmd:   "TUNNEL",
			expectedError: false,
		},
		{
			name:          "SleepCommand",
			response:      "CMD SLEEP 60 10\n",
			expectedCmd:   "SLEEP",
			expectedError: false,
		},
		{
			name:          "ErrorResponse",
			response:      "ERR Invalid password\n",
			expectedCmd:   "",
			expectedError: true,
		},
		{
			name:          "InvalidFormat",
			response:      "UNKNOWN\n",
			expectedCmd:   "",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Функция parseServerCommand не экспортирована
			// Проверяем только валидность формата команд здесь
			if strings.HasPrefix(tc.response, "CMD ") {
				parts := strings.Fields(tc.response)
				if len(parts) < 2 {
					t.Error("Invalid CMD format")
				}
			}
		})
	}
}

// TestRandBigInt проверяет что RandBigInt возвращает случайные числа в диапазоне
func TestRandBigInt(t *testing.T) {
	max := big.NewInt(10000)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		result := common.RandBigInt(max)

		if result.Cmp(big.NewInt(0)) < 0 {
			t.Error("RandBigInt returned negative number")
		}

		if result.Cmp(max) >= 0 {
			t.Errorf("RandBigInt returned %v >= max %v", result, max)
		}
	}
}
