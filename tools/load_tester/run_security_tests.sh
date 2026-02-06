#!/usr/bin/env bash
#
# Быстрый pre-production тест RevSocks сервера.
# Запускает все критические проверки последовательно.
#

set -euo pipefail

# ============================================================================
# КОНФИГУРАЦИЯ
# ============================================================================

SERVER_HOST="${SERVER_HOST:-192.168.1.108}"
SERVER_PORT="${SERVER_PORT:-10443}"
ADMIN_PORT="${ADMIN_PORT:-8081}"
SERVER_PASS="${SERVER_PASS:-SFOpkm3rffAds90SF3ghSD}"
USE_TLS="${USE_TLS:-1}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTER_SCRIPT="$SCRIPT_DIR/server_stress_test.py"

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# ФУНКЦИИ
# ============================================================================

log_info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

log_success() {
    echo -e "${GREEN}✓${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*"
}

separator() {
    echo ""
    echo "======================================================================"
    echo ""
}

check_dependencies() {
    log_info "Проверка зависимостей..."
    
    if ! command -v python3 &> /dev/null; then
        log_error "python3 не найден!"
        exit 1
    fi
    
    if ! python3 -c "import requests" 2>/dev/null; then
        log_error "Модуль requests не установлен!"
        echo "Установите: pip3 install requests websocket-client"
        exit 1
    fi
    
    if ! python3 -c "import websocket" 2>/dev/null; then
        log_error "Модуль websocket-client не установлен!"
        echo "Установите: pip3 install websocket-client"
        exit 1
    fi
    
    if [ ! -f "$TESTER_SCRIPT" ]; then
        log_error "Скрипт тестирования не найден: $TESTER_SCRIPT"
        exit 1
    fi
    
    log_success "Все зависимости на месте"
}

check_server() {
    log_info "Проверка доступности сервера..."
    
    # Проверяем основной порт
    if timeout 2 bash -c "echo > /dev/tcp/$SERVER_HOST/$SERVER_PORT" 2>/dev/null; then
        log_success "Сервер отвечает на $SERVER_HOST:$SERVER_PORT"
    else
        log_error "Сервер недоступен на $SERVER_HOST:$SERVER_PORT"
        log_warning "Запустите сервер командой:"
        echo "  ./revsocks-server -listen :$SERVER_PORT -socks 127.0.0.1:1080 -pass '$SERVER_PASS' -tls -admin-api -admin-port :$ADMIN_PORT -ws"
        exit 1
    fi
    
    # Проверяем Admin API
    if timeout 2 bash -c "echo > /dev/tcp/$SERVER_HOST/$ADMIN_PORT" 2>/dev/null; then
        log_success "Admin API отвечает на $SERVER_HOST:$ADMIN_PORT"
    else
        log_warning "Admin API недоступен (это не критично)"
    fi
}

run_test() {
    local mode=$1
    local description=$2
    local extra_args=${3:-}
    
    separator
    log_info "🎯 ТЕСТ: $description"
    separator
    
    local cmd="python3 '$TESTER_SCRIPT' \
        --host '$SERVER_HOST' \
        --port '$SERVER_PORT' \
        --admin-port '$ADMIN_PORT' \
        --password '$SERVER_PASS' \
        --mode '$mode' \
        $extra_args"
    
    if [ "$USE_TLS" == "1" ]; then
        cmd="$cmd --tls"
    fi
    
    if eval "$cmd"; then
        log_success "Тест '$mode' завершен успешно"
        return 0
    else
        log_error "Тест '$mode' завершился с ошибкой!"
        return 1
    fi
}

post_test_check() {
    log_info "Проверка состояния после теста..."
    
    # Проверяем что сервер все еще отвечает
    if timeout 2 bash -c "echo > /dev/tcp/$SERVER_HOST/$SERVER_PORT" 2>/dev/null; then
        log_success "Сервер все еще отвечает"
    else
        log_error "⚠️ СЕРВЕР НЕ ОТВЕЧАЕТ! Возможно упал!"
        return 1
    fi
    
    # Считаем открытые соединения
    local conn_count
    conn_count=$(netstat -an 2>/dev/null | grep ":$SERVER_PORT" | wc -l || echo "N/A")
    log_info "Открытых соединений: $conn_count"
    
    if [ "$conn_count" != "N/A" ] && [ "$conn_count" -gt 100 ]; then
        log_warning "Много открытых соединений! Возможна утечка дескрипторов"
    fi
    
    return 0
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════╗"
    echo "║         RevSocks Pre-Production Security Test Suite               ║"
    echo "╚════════════════════════════════════════════════════════════════════╝"
    echo ""
    
    log_info "Конфигурация:"
    echo "  Server:     $SERVER_HOST:$SERVER_PORT"
    echo "  Admin API:  $SERVER_HOST:$ADMIN_PORT"
    echo "  TLS:        $([ "$USE_TLS" == "1" ] && echo "Enabled" || echo "Disabled")"
    echo ""
    
    log_warning "⚠️  ВНИМАНИЕ: Это агрессивное тестирование!"
    log_warning "   Используйте только в тестовом окружении!"
    echo ""
    
    read -p "Продолжить? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Отменено пользователем"
        exit 0
    fi
    
    separator
    
    # Предварительные проверки
    check_dependencies
    check_server
    
    separator
    log_info "Начало тестирования..."
    separator
    
    local failed_tests=0
    
    # Тест 1: Флуд авторизацией
    if ! run_test "auth_flood" "Флуд валидными подключениями" "--workers 15 --iterations 50"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Тест 2: Неверные пароли
    if ! run_test "invalid_auth" "Флуд неверными паролями" "--workers 20 --iterations 100"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Тест 3: Некорректные данные
    if ! run_test "malformed_data" "Отправка мусорных данных" "--workers 10"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Тест 4: Медленные клиенты
    if ! run_test "slowloris" "Медленные клиенты (Slowloris)" "--workers 20"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Тест 5: Быстрые переподключения
    if ! run_test "rapid_reconnect" "Быстрые переподключения" "--workers 10 --iterations 150"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Тест 6: Admin API
    if ! run_test "admin_api" "Атаки на Admin API" "--workers 5"; then
        ((failed_tests++))
    fi
    sleep 2
    post_test_check || ((failed_tests++))
    
    # Итоговый отчет
    separator
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════╗"
    echo "║                        ИТОГОВЫЙ ОТЧЕТ                              ║"
    echo "╚════════════════════════════════════════════════════════════════════╝"
    echo ""
    
    if [ $failed_tests -eq 0 ]; then
        log_success "ВСЕ ТЕСТЫ ПРОЙДЕНЫ!"
        echo ""
        log_success "✓ Сервер устойчив к флуду"
        log_success "✓ Авторизация защищена"
        log_success "✓ Некорректные данные обрабатываются"
        log_success "✓ Таймауты работают корректно"
        log_success "✓ SessionManager стабилен"
        log_success "✓ Admin API защищен"
        echo ""
        log_info "Сервер готов к продакшену! 🚀"
    else
        log_error "ОБНАРУЖЕНЫ ПРОБЛЕМЫ!"
        echo ""
        log_error "Провалено тестов: $failed_tests"
        echo ""
        log_warning "РЕКОМЕНДАЦИИ:"
        echo "  1. Проверьте логи сервера на панику/ошибки"
        echo "  2. Проверьте использование памяти: ps aux | grep revsocks"
        echo "  3. Проверьте соединения: netstat -an | grep $SERVER_PORT"
        echo "  4. Попробуйте подключиться валидным агентом"
        echo ""
        log_error "НЕ РАЗВОРАЧИВАЙТЕ В ПРОДАКШЕН!"
    fi
    
    separator
    
    exit $failed_tests
}

# Обработка Ctrl+C
trap 'echo ""; log_warning "Прервано пользователем"; exit 130' INT

main "$@"
