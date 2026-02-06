#!/bin/bash
#
# build.sh - Система сборки RevSocks (Codegen)
#
# Режимы:
#   ./build.sh              - Обычная сборка агента
#   ./build.sh stealth      - Stealth сборка с baked config
#   ./build.sh server       - Сборка сервера
#   ./build.sh all          - Сборка агента + сервера
#   ./build.sh clean        - Очистка
#

set -e

# Цвета
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Параметры
MODE="${1:-normal}"
CONFIG_FILE="config/revsocks.yaml"
BAKED_FILE="internal/agent/baked.go"

# Парсим флаг --config
shift || true
while [[ $# -gt 0 ]]; do
    case $1 in
        --config|-c)
            CONFIG_FILE="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Версия
VERSION=$(grep 'VERSION=' Makefile 2>/dev/null | head -1 | cut -d'=' -f2 || echo "1.0.0")
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null | cut -c1-7 || echo "unknown")

# Баннер
echo -e "${BLUE}"
echo "========================================"
echo "   RevSocks Build System v3.0          "
echo "   Codegen Architecture                "
echo "========================================"
echo -e "${NC}"

# ========================================
# Функции
# ========================================

build_confgen() {
    log_info "Проверяю tools/confgen..."
    if ! go build -o /dev/null ./tools/confgen 2>/dev/null; then
        log_error "tools/confgen не компилируется!"
        exit 1
    fi
}

generate_baked() {
    log_info "Генерирую baked.go из $CONFIG_FILE..."
    
    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "Конфиг $CONFIG_FILE не найден!"
        log_info "Создайте конфиг: cp config/revsocks.yaml.example config/revsocks.yaml"
        exit 1
    fi
    
    GOOS=linux GOARCH=amd64 go run ./tools/confgen -config "$CONFIG_FILE" -out "$BAKED_FILE"
    
    if [ $? -ne 0 ]; then
        log_error "Ошибка генерации baked.go!"
        exit 1
    fi
}

reset_baked() {
    log_info "Сбрасываю baked.go на дефолтный..."
    GOOS=linux GOARCH=amd64 go run ./tools/confgen -default -out "$BAKED_FILE"
}

build_agent() {
    local output_name="$1"
    local stealth_suffix=""
    
    if [ "$MODE" = "stealth" ]; then
        stealth_suffix="-stealth"
    fi
    
    log_info "Компилирую агента..."
    
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=${VERSION}${stealth_suffix} -X github.com/kost/revsocks/internal/common.CommitID=${GIT_COMMIT}" \
        -o "$output_name" ./cmd/agent
    
    log_success "Агент собран: $output_name ($(du -h "$output_name" | cut -f1))"
}

build_server() {
    log_info "Компилирую сервер..."
    
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -X github.com/kost/revsocks/internal/common.Version=${VERSION} -X github.com/kost/revsocks/internal/common.CommitID=${GIT_COMMIT}" \
        -o "revsocks-server" ./cmd/server
    
    log_success "Сервер собран: revsocks-server ($(du -h revsocks-server | cut -f1))"
}

apply_upx() {
    local binary="$1"
    
    if ! command -v upx &> /dev/null; then
        log_warning "UPX не установлен, пропускаю сжатие"
        return
    fi
    
    log_info "UPX сжатие..."
    upx --best -7 --lzma "$binary" 2>&1 | grep -v "Packed\|compressed" || true
    log_success "Сжато: $binary ($(du -h "$binary" | cut -f1))"
}

# ========================================
# Режимы сборки
# ========================================

case "$MODE" in
    normal|agent)
        log_info "Режим: ОБЫЧНАЯ СБОРКА"
        reset_baked
        build_agent "revsocks-agent"
        ;;
        
    stealth)
        log_info "Режим: STEALTH СБОРКА"
        
        build_confgen
        generate_baked
        
        # Имя выходного файла из конфига
        OUTPUT_NAME=$(grep 'output_name:' "$CONFIG_FILE" | awk '{print $2}' | tr -d '"' || echo "revsocks-agent-stealth")
        
        build_agent "$OUTPUT_NAME"
        
        # UPX если включён
        UPX_ENABLED=$(grep -A2 'upx:' "$CONFIG_FILE" | grep 'enabled:' | awk '{print $2}' || echo "false")
        if [ "$UPX_ENABLED" = "true" ]; then
            apply_upx "$OUTPUT_NAME"
        fi
        
        # Восстанавливаем дефолтный baked.go
        reset_baked
        
        log_success "Stealth агент готов: $OUTPUT_NAME"
        ;;
        
    server)
        log_info "Режим: СБОРКА СЕРВЕРА"
        build_server
        ;;
        
    all)
        log_info "Режим: ПОЛНАЯ СБОРКА"
        reset_baked
        build_agent "revsocks-agent"
        build_server
        ;;
        
    clean)
        log_info "Очистка..."
        rm -f revsocks-agent revsocks-server
        rm -f *.backup
        reset_baked
        log_success "Очищено"
        ;;
        
    *)
        echo "Использование: $0 [MODE] [OPTIONS]"
        echo ""
        echo "Режимы:"
        echo "  normal, agent   Обычная сборка агента"
        echo "  stealth         Stealth сборка с baked config"
        echo "  server          Сборка сервера"
        echo "  all             Сборка агента и сервера"
        echo "  clean           Очистка"
        echo ""
        echo "Опции:"
        echo "  --config, -c    Путь к конфигу (по умолчанию: config/revsocks.yaml)"
        echo ""
        echo "Примеры:"
        echo "  $0 stealth              # Stealth сборка"
        echo "  $0 all                  # Агент + сервер"
        exit 0
        ;;
esac

echo ""
log_success "Сборка завершена!"
