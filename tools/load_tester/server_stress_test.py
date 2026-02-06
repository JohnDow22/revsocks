#!/usr/bin/env python3
"""
Стресс-тестирование серверной части revsocks.

Проверяет устойчивость сервера к различным типам атак и edge cases:
- Флуд подключениями
- Неправильная аутентификация
- Некорректные данные
- Медленные клиенты (slowloris)
- Разрывы соединений
- WebSocket атаки
- Admin API атаки
"""

import argparse
import asyncio
import json
import random
import signal
import ssl
import struct
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Tuple
from enum import Enum

import websocket
import requests
from requests.packages.urllib3.exceptions import InsecureRequestWarning

# Отключаем предупреждения о самоподписанных сертификатах
requests.packages.urllib3.disable_warnings(InsecureRequestWarning)


# ============================================================================
# КОНФИГУРАЦИЯ
# ============================================================================

DEFAULT_SERVER_HOST = "192.168.1.108"
DEFAULT_SERVER_PORT = 10443
DEFAULT_ADMIN_PORT = 8081
DEFAULT_PASSWORD = "SFOpkm3rffAds90SF3ghSD"

# Режимы тестирования
class TestMode(Enum):
    AUTH_FLOOD = "auth_flood"           # Флуд попытками авторизации
    CONNECTION_FLOOD = "conn_flood"     # Флуд соединениями
    INVALID_AUTH = "invalid_auth"       # Неверные пароли
    MALFORMED_DATA = "malformed_data"   # Некорректные данные
    SLOWLORIS = "slowloris"             # Медленные клиенты
    RAPID_RECONNECT = "rapid_reconnect" # Быстрые переподключения
    ADMIN_API_ATTACK = "admin_api"      # Атака на Admin API
    MIXED = "mixed"                     # Смешанная атака
    ALL = "all"                         # Все режимы последовательно


# ============================================================================
# РЕЗУЛЬТАТЫ
# ============================================================================

@dataclass
class AttackResult:
    """Результат одной атаки."""
    attack_type: str
    success: bool
    duration_ms: float
    error_msg: Optional[str] = None
    details: Dict = field(default_factory=dict)


@dataclass
class TestStats:
    """Статистика теста."""
    test_name: str
    total: int
    success: int
    failed: int
    avg_time_ms: float
    min_time_ms: float
    max_time_ms: float
    timestamp: str
    details: str = ""


# ============================================================================
# ГЛОБАЛЬНОЕ СОСТОЯНИЕ
# ============================================================================

class GlobalState:
    """Глобальное состояние."""
    shutdown_requested = False
    total_attacks = 0
    total_success = 0
    total_failed = 0


# ============================================================================
# АТАКА 1: ФЛУД АВТОРИЗАЦИЕЙ
# ============================================================================

def attack_auth_flood(
    host: str,
    port: int,
    password: str,
    use_tls: bool,
    iterations: int = 100
) -> AttackResult:
    """
    Флуд попытками авторизации через WebSocket.
    
    Цель: Проверить устойчивость к множественным попыткам подключения
    с валидным паролем (создание множества агентов).
    """
    start_time = time.time()
    connections_created = 0
    connections_failed = 0
    
    try:
        for i in range(iterations):
            if GlobalState.shutdown_requested:
                break
                
            try:
                # Подключаемся через WebSocket
                ws_url = f"{'wss' if use_tls else 'ws'}://{host}:{port}"
                ws = websocket.create_connection(
                    ws_url,
                    timeout=5,
                    sslopt={"cert_reqs": ssl.CERT_NONE} if use_tls else None
                )
                
                # Отправляем пароль
                ws.send(password + "\n")
                
                # Читаем ответ (если есть)
                try:
                    response = ws.recv()
                    connections_created += 1
                except:
                    connections_created += 1
                
                # НЕ закрываем соединение сразу - держим открытым
                # (проверка утечек памяти/дескрипторов)
                
            except Exception as e:
                connections_failed += 1
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="auth_flood",
            success=True,
            duration_ms=duration_ms,
            details={
                "created": connections_created,
                "failed": connections_failed,
                "leaked": connections_created  # Оставили открытыми
            }
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="auth_flood",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# АТАКА 2: НЕВЕРНЫЕ ПАРОЛИ
# ============================================================================

def attack_invalid_auth(
    host: str,
    port: int,
    use_tls: bool,
    iterations: int = 50
) -> AttackResult:
    """
    Флуд с неверными паролями.
    
    Цель: Проверить обработку неудачной авторизации и защиту от брутфорса.
    """
    start_time = time.time()
    attempts = 0
    rejected = 0
    
    try:
        for i in range(iterations):
            if GlobalState.shutdown_requested:
                break
                
            try:
                ws_url = f"{'wss' if use_tls else 'ws'}://{host}:{port}"
                ws = websocket.create_connection(
                    ws_url,
                    timeout=3,
                    sslopt={"cert_reqs": ssl.CERT_NONE} if use_tls else None
                )
                
                # Отправляем НЕВЕРНЫЙ пароль
                fake_pass = f"wrong_password_{random.randint(1000, 9999)}"
                ws.send(fake_pass + "\n")
                
                try:
                    ws.recv()
                except:
                    rejected += 1
                    
                ws.close()
                attempts += 1
                
            except Exception:
                rejected += 1
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="invalid_auth",
            success=True,
            duration_ms=duration_ms,
            details={
                "attempts": attempts,
                "rejected": rejected
            }
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="invalid_auth",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# АТАКА 3: НЕКОРРЕКТНЫЕ ДАННЫЕ
# ============================================================================

def attack_malformed_data(
    host: str,
    port: int,
    use_tls: bool
) -> AttackResult:
    """
    Отправка некорректных данных в различных стадиях.
    
    Цель: Проверить парсинг и обработку ошибок протокола.
    """
    start_time = time.time()
    malformed_payloads = [
        b"\x00" * 1000,                          # Нули
        b"\xff" * 1000,                          # Мусор
        b"GET / HTTP/1.1\r\n\r\n",              # HTTP вместо WebSocket
        b"random garbage data \x00\xff\xaa",    # Случайные байты
        struct.pack(">H", 0xffff) * 500,        # Большие числа
        b"\n" * 1000,                            # Переносы строк
        password.encode() * 100,                 # Множество паролей
    ]
    
    attempts = 0
    
    try:
        for payload in malformed_payloads:
            if GlobalState.shutdown_requested:
                break
                
            try:
                ws_url = f"{'wss' if use_tls else 'ws'}://{host}:{port}"
                ws = websocket.create_connection(
                    ws_url,
                    timeout=3,
                    sslopt={"cert_reqs": ssl.CERT_NONE} if use_tls else None
                )
                
                # Отправляем мусорные данные
                ws.send(payload, opcode=websocket.ABNF.OPCODE_BINARY)
                
                try:
                    ws.recv()
                except:
                    pass
                    
                ws.close()
                attempts += 1
                
            except Exception:
                attempts += 1
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="malformed_data",
            success=True,
            duration_ms=duration_ms,
            details={"payloads_sent": attempts}
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="malformed_data",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# АТАКА 4: SLOWLORIS
# ============================================================================

def attack_slowloris(
    host: str,
    port: int,
    use_tls: bool,
    connections: int = 20
) -> AttackResult:
    """
    Медленные клиенты (slowloris-подобная атака).
    
    Цель: Проверить таймауты и защиту от медленных клиентов.
    Открываем соединения и отправляем данные очень медленно.
    """
    start_time = time.time()
    open_connections = []
    
    try:
        # Открываем множество соединений
        for i in range(connections):
            if GlobalState.shutdown_requested:
                break
                
            try:
                ws_url = f"{'wss' if use_tls else 'ws'}://{host}:{port}"
                ws = websocket.create_connection(
                    ws_url,
                    timeout=30,
                    sslopt={"cert_reqs": ssl.CERT_NONE} if use_tls else None
                )
                open_connections.append(ws)
            except:
                pass
        
        # Держим соединения открытыми и отправляем данные по байту
        for i in range(10):
            if GlobalState.shutdown_requested:
                break
                
            for ws in open_connections:
                try:
                    # Отправляем по одному байту
                    ws.send(b"\x00", opcode=websocket.ABNF.OPCODE_BINARY)
                except:
                    pass
                    
            time.sleep(2)  # Ждем между отправками
        
        # Закрываем соединения
        for ws in open_connections:
            try:
                ws.close()
            except:
                pass
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="slowloris",
            success=True,
            duration_ms=duration_ms,
            details={"connections": len(open_connections)}
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="slowloris",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# АТАКА 5: БЫСТРЫЕ ПЕРЕПОДКЛЮЧЕНИЯ
# ============================================================================

def attack_rapid_reconnect(
    host: str,
    port: int,
    password: str,
    use_tls: bool,
    iterations: int = 50
) -> AttackResult:
    """
    Быстрые переподключения одного агента.
    
    Цель: Проверить SessionManager и cleanup логику.
    Один agent_id постоянно переподключается.
    """
    start_time = time.time()
    reconnects = 0
    agent_id = f"test_agent_{random.randint(1000, 9999)}"
    
    try:
        for i in range(iterations):
            if GlobalState.shutdown_requested:
                break
                
            try:
                ws_url = f"{'wss' if use_tls else 'ws'}://{host}:{port}"
                ws = websocket.create_connection(
                    ws_url,
                    timeout=3,
                    sslopt={"cert_reqs": ssl.CERT_NONE} if use_tls else None
                )
                
                # Отправляем пароль + agent_id
                auth_data = f"{password}\n{agent_id}"
                ws.send(auth_data)
                
                try:
                    ws.recv()
                except:
                    pass
                    
                # Сразу закрываем
                ws.close()
                reconnects += 1
                
                # Минимальная задержка
                time.sleep(0.05)
                
            except Exception:
                pass
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="rapid_reconnect",
            success=True,
            duration_ms=duration_ms,
            details={
                "reconnects": reconnects,
                "agent_id": agent_id
            }
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="rapid_reconnect",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# АТАКА 6: ADMIN API
# ============================================================================

def attack_admin_api(
    host: str,
    admin_port: int,
    use_tls: bool
) -> AttackResult:
    """
    Атака на Admin API.
    
    Цель: Проверить защиту Admin API и обработку некорректных запросов.
    """
    start_time = time.time()
    attacks = []
    
    base_url = f"{'https' if use_tls else 'http'}://{host}:{admin_port}"
    
    # Различные типы атак на API
    test_cases = [
        # SQL injection попытки
        ("GET", "/api/agents?id=' OR '1'='1", {}),
        ("GET", "/api/sessions?filter=1; DROP TABLE sessions;--", {}),
        
        # Path traversal
        ("GET", "/api/../../../etc/passwd", {}),
        ("GET", "/api/agents/../../config", {}),
        
        # Большие запросы
        ("POST", "/api/agents", {"data": "A" * 1000000}),
        
        # Некорректные методы
        ("DELETE", "/api/agents/all", {}),
        ("PUT", "/api/config", {"dangerous": "value"}),
        
        # Некорректный JSON
        ("POST", "/api/agents", "not a json"),
        
        # XSS попытки
        ("GET", "/api/agents?name=<script>alert(1)</script>", {}),
    ]
    
    try:
        for method, path, data in test_cases:
            if GlobalState.shutdown_requested:
                break
                
            try:
                url = base_url + path
                
                if method == "GET":
                    response = requests.get(url, timeout=5, verify=False)
                elif method == "POST":
                    response = requests.post(url, json=data, timeout=5, verify=False)
                elif method == "DELETE":
                    response = requests.delete(url, timeout=5, verify=False)
                elif method == "PUT":
                    response = requests.put(url, json=data, timeout=5, verify=False)
                    
                attacks.append({
                    "method": method,
                    "path": path,
                    "status": response.status_code
                })
                
            except Exception as e:
                attacks.append({
                    "method": method,
                    "path": path,
                    "error": str(e)
                })
                
        duration_ms = (time.time() - start_time) * 1000
        
        return AttackResult(
            attack_type="admin_api",
            success=True,
            duration_ms=duration_ms,
            details={"attacks": len(attacks)}
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return AttackResult(
            attack_type="admin_api",
            success=False,
            duration_ms=duration_ms,
            error_msg=str(e)
        )


# ============================================================================
# ПАРАЛЛЕЛЬНОЕ ВЫПОЛНЕНИЕ
# ============================================================================

def run_parallel_attacks(
    attack_func,
    workers: int,
    **kwargs
) -> List[AttackResult]:
    """Запускает атаки параллельно в нескольких потоках."""
    results = []
    
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [
            executor.submit(attack_func, **kwargs)
            for _ in range(workers)
        ]
        
        for future in as_completed(futures):
            result = future.result()
            results.append(result)
            
    return results


# ============================================================================
# СТАТИСТИКА
# ============================================================================

def calculate_stats(test_name: str, results: List[AttackResult]) -> TestStats:
    """Вычисляет статистику по результатам."""
    total = len(results)
    success = sum(1 for r in results if r.success)
    failed = total - success
    
    durations = [r.duration_ms for r in results]
    avg_time = sum(durations) / len(durations) if durations else 0
    min_time = min(durations) if durations else 0
    max_time = max(durations) if durations else 0
    
    # Собираем детали
    details_list = []
    for r in results:
        if r.details:
            details_list.append(str(r.details))
    details_str = "; ".join(details_list[:3])  # Первые 3
    
    return TestStats(
        test_name=test_name,
        total=total,
        success=success,
        failed=failed,
        avg_time_ms=avg_time,
        min_time_ms=min_time,
        max_time_ms=max_time,
        timestamp=datetime.now().strftime("%H:%M:%S"),
        details=details_str
    )


def print_stats(stats: TestStats):
    """Выводит статистику теста."""
    success_rate = (stats.success / stats.total * 100) if stats.total > 0 else 0
    
    print(f"\n{'='*70}")
    print(f"[{stats.timestamp}] 🎯 {stats.test_name}")
    print(f"{'='*70}")
    print(f"Total:    {stats.total}")
    print(f"Success:  {stats.success} ({success_rate:.1f}%)")
    print(f"Failed:   {stats.failed}")
    print(f"Timing:   AVG={stats.avg_time_ms:.0f}ms MIN={stats.min_time_ms:.0f}ms MAX={stats.max_time_ms:.0f}ms")
    
    if stats.details:
        print(f"Details:  {stats.details}")
    
    print(f"{'='*70}")


# ============================================================================
# GRACEFUL SHUTDOWN
# ============================================================================

def signal_handler(signum, frame):
    """Обработчик Ctrl+C."""
    print("\n\n🛑 Прерывание по Ctrl+C. Завершение...")
    GlobalState.shutdown_requested = True


# ============================================================================
# MAIN
# ============================================================================

def main():
    """Основная функция."""
    parser = argparse.ArgumentParser(
        description="Стресс-тестирование серверной части RevSocks"
    )
    
    parser.add_argument(
        "--host",
        default=DEFAULT_SERVER_HOST,
        help=f"Хост сервера (по умолчанию: {DEFAULT_SERVER_HOST})"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=DEFAULT_SERVER_PORT,
        help=f"Порт сервера (по умолчанию: {DEFAULT_SERVER_PORT})"
    )
    parser.add_argument(
        "--admin-port",
        type=int,
        default=DEFAULT_ADMIN_PORT,
        help=f"Порт Admin API (по умолчанию: {DEFAULT_ADMIN_PORT})"
    )
    parser.add_argument(
        "--password",
        default=DEFAULT_PASSWORD,
        help="Пароль сервера"
    )
    parser.add_argument(
        "--tls",
        action="store_true",
        help="Использовать TLS/WSS"
    )
    parser.add_argument(
        "--mode",
        choices=[m.value for m in TestMode],
        default=TestMode.ALL.value,
        help="Режим тестирования"
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=10,
        help="Количество параллельных потоков"
    )
    parser.add_argument(
        "--iterations",
        type=int,
        default=50,
        help="Количество итераций для каждого теста"
    )
    
    args = parser.parse_args()
    
    # Глобальная переменная для доступа к паролю
    global password
    password = args.password
    
    # Регистрируем обработчик Ctrl+C
    signal.signal(signal.SIGINT, signal_handler)
    
    # Выводим конфигурацию
    print("=" * 70)
    print("🔥 RevSocks Server Stress Tester")
    print("=" * 70)
    print(f"Server:      {args.host}:{args.port}")
    print(f"Admin API:   {args.host}:{args.admin_port}")
    print(f"TLS:         {'Enabled' if args.tls else 'Disabled'}")
    print(f"Mode:        {args.mode}")
    print(f"Workers:     {args.workers}")
    print(f"Iterations:  {args.iterations}")
    print("=" * 70)
    print("⚠️  ВНИМАНИЕ: Это агрессивное тестирование!")
    print("   Сервер может упасть, зависнуть или деградировать.")
    print("   Используйте только в тестовом окружении!")
    print("=" * 70)
    print("\nНажмите Ctrl+C для остановки\n")
    
    time.sleep(2)  # Даем время прочитать
    
    # Выбираем тесты для запуска
    tests_to_run = []
    
    if args.mode == TestMode.ALL.value:
        tests_to_run = [
            TestMode.AUTH_FLOOD,
            TestMode.INVALID_AUTH,
            TestMode.MALFORMED_DATA,
            TestMode.SLOWLORIS,
            TestMode.RAPID_RECONNECT,
            TestMode.ADMIN_API_ATTACK
        ]
    else:
        tests_to_run = [TestMode(args.mode)]
    
    # Запускаем тесты
    for test_mode in tests_to_run:
        if GlobalState.shutdown_requested:
            break
            
        print(f"\n🚀 Запуск: {test_mode.value.upper()}")
        
        try:
            if test_mode == TestMode.AUTH_FLOOD:
                results = run_parallel_attacks(
                    attack_auth_flood,
                    workers=args.workers,
                    host=args.host,
                    port=args.port,
                    password=args.password,
                    use_tls=args.tls,
                    iterations=args.iterations
                )
                stats = calculate_stats("Auth Flood Attack", results)
                
            elif test_mode == TestMode.INVALID_AUTH:
                results = run_parallel_attacks(
                    attack_invalid_auth,
                    workers=args.workers,
                    host=args.host,
                    port=args.port,
                    use_tls=args.tls,
                    iterations=args.iterations
                )
                stats = calculate_stats("Invalid Auth Attack", results)
                
            elif test_mode == TestMode.MALFORMED_DATA:
                results = run_parallel_attacks(
                    attack_malformed_data,
                    workers=args.workers,
                    host=args.host,
                    port=args.port,
                    use_tls=args.tls
                )
                stats = calculate_stats("Malformed Data Attack", results)
                
            elif test_mode == TestMode.SLOWLORIS:
                results = run_parallel_attacks(
                    attack_slowloris,
                    workers=args.workers,
                    host=args.host,
                    port=args.port,
                    use_tls=args.tls,
                    connections=20
                )
                stats = calculate_stats("Slowloris Attack", results)
                
            elif test_mode == TestMode.RAPID_RECONNECT:
                results = run_parallel_attacks(
                    attack_rapid_reconnect,
                    workers=args.workers,
                    host=args.host,
                    port=args.port,
                    password=args.password,
                    use_tls=args.tls,
                    iterations=args.iterations
                )
                stats = calculate_stats("Rapid Reconnect Attack", results)
                
            elif test_mode == TestMode.ADMIN_API_ATTACK:
                results = run_parallel_attacks(
                    attack_admin_api,
                    workers=args.workers,
                    host=args.host,
                    admin_port=args.admin_port,
                    use_tls=args.tls
                )
                stats = calculate_stats("Admin API Attack", results)
            
            print_stats(stats)
            
            GlobalState.total_attacks += stats.total
            GlobalState.total_success += stats.success
            GlobalState.total_failed += stats.failed
            
            # Пауза между тестами
            if not GlobalState.shutdown_requested:
                print("\n⏸️  Пауза 3 секунды перед следующим тестом...")
                time.sleep(3)
                
        except Exception as e:
            print(f"\n❌ Ошибка при выполнении теста {test_mode.value}: {e}")
    
    # Итоговая статистика
    print("\n" + "=" * 70)
    print("📊 ИТОГОВАЯ СТАТИСТИКА")
    print("=" * 70)
    print(f"Всего атак:     {GlobalState.total_attacks}")
    print(f"Успешных:       {GlobalState.total_success}")
    print(f"Неудачных:      {GlobalState.total_failed}")
    
    if GlobalState.total_attacks > 0:
        success_rate = (GlobalState.total_success / GlobalState.total_attacks * 100)
        print(f"Success rate:   {success_rate:.1f}%")
    
    print("=" * 70)
    print("\n✅ Тестирование завершено.")
    print("\n💡 РЕКОМЕНДАЦИИ:")
    print("   1. Проверьте логи сервера на панику/ошибки")
    print("   2. Проверьте использование памяти (ps aux)")
    print("   3. Проверьте открытые соединения (netstat -an)")
    print("   4. Проверьте Admin API (/api/sessions, /api/agents)")
    print("   5. Попробуйте подключиться валидным агентом")


if __name__ == "__main__":
    main()
