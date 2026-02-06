#!/usr/bin/env python3
"""
Инструмент нагрузочного тестирования для revsocks прокси.

Генерирует burst-нагрузку (пачки параллельных запросов) для проверки 
стабильности работы прокси при множественных подключениях.
"""

import argparse
import signal
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import datetime
from typing import Optional, Tuple

import requests


# ============================================================================
# КОНФИГУРАЦИЯ ПО УМОЛЧАНИЮ
# ============================================================================

DEFAULT_PROXY_HOST = "192.168.1.108"
DEFAULT_PROXY_PORT = 60281
DEFAULT_PROXY_USER = "test"
DEFAULT_PROXY_PASS = "test"
DEFAULT_TARGET_URL = "https://google.com"

DEFAULT_THREADS = 10       # Количество потоков в одном burst
DEFAULT_INTERVAL = 1.0     # Интервал между burst (секунды)
DEFAULT_TIMEOUT = 10       # Таймаут подключения (секунды)


# ============================================================================
# СТРУКТУРЫ ДАННЫХ
# ============================================================================

@dataclass
class RequestResult:
    """Результат одного HTTP запроса через прокси."""
    success: bool
    duration_ms: float
    error_msg: Optional[str] = None
    status_code: Optional[int] = None


@dataclass
class BatchStats:
    """Статистика одной пачки (burst) запросов."""
    iteration: int
    total: int
    success: int
    failed: int
    avg_time_ms: float
    min_time_ms: float
    max_time_ms: float
    timestamp: str


# ============================================================================
# ГЛОБАЛЬНОЕ СОСТОЯНИЕ
# ============================================================================

class GlobalState:
    """Глобальное состояние для graceful shutdown."""
    shutdown_requested = False


# ============================================================================
# WORKER: ВЫПОЛНЕНИЕ ОДНОГО ЗАПРОСА
# ============================================================================

def make_request(
    request_id: int,
    proxy_url: str,
    target_url: str,
    timeout: int
) -> RequestResult:
    """
    Выполняет один HTTP GET запрос через SOCKS5 прокси.
    
    Шаги:
    1. Настраивает SOCKS5 прокси для requests
    2. Выполняет GET запрос к целевому URL
    3. Замеряет время выполнения
    4. Возвращает результат (успех/ошибка)
    
    Args:
        request_id: ID запроса (для логирования)
        proxy_url: URL прокси в формате socks5://user:pass@host:port
        target_url: Целевой URL для запроса
        timeout: Таймаут в секундах
        
    Returns:
        RequestResult с результатами выполнения
    """
    start_time = time.time()
    
    try:
        proxies = {
            'http': proxy_url,
            'https': proxy_url
        }
        
        response = requests.get(
            target_url,
            proxies=proxies,
            timeout=timeout,
            allow_redirects=True
        )
        
        duration_ms = (time.time() - start_time) * 1000
        
        return RequestResult(
            success=True,
            duration_ms=duration_ms,
            status_code=response.status_code
        )
        
    except requests.exceptions.Timeout:
        duration_ms = (time.time() - start_time) * 1000
        return RequestResult(
            success=False,
            duration_ms=duration_ms,
            error_msg="Timeout"
        )
        
    except requests.exceptions.ProxyError as e:
        duration_ms = (time.time() - start_time) * 1000
        return RequestResult(
            success=False,
            duration_ms=duration_ms,
            error_msg=f"Proxy Error: {str(e)}"
        )
        
    except Exception as e:
        duration_ms = (time.time() - start_time) * 1000
        return RequestResult(
            success=False,
            duration_ms=duration_ms,
            error_msg=f"Error: {str(e)}"
        )


# ============================================================================
# BURST GENERATOR: ЗАПУСК ПАЧКИ ЗАПРОСОВ
# ============================================================================

def run_burst(
    iteration: int,
    threads: int,
    proxy_url: str,
    target_url: str,
    timeout: int
) -> BatchStats:
    """
    Выполняет одну пачку (burst) параллельных запросов.
    
    Шаги:
    1. Создает ThreadPoolExecutor с заданным количеством потоков
    2. Запускает все запросы одновременно
    3. Собирает результаты
    4. Вычисляет статистику (успех/ошибка, времена)
    
    Args:
        iteration: Номер итерации
        threads: Количество параллельных потоков
        proxy_url: URL прокси
        target_url: Целевой URL
        timeout: Таймаут запросов
        
    Returns:
        BatchStats со статистикой пачки
    """
    results = []
    
    with ThreadPoolExecutor(max_workers=threads) as executor:
        # Запускаем все запросы параллельно
        futures = {
            executor.submit(make_request, i, proxy_url, target_url, timeout): i
            for i in range(threads)
        }
        
        # Собираем результаты по мере завершения
        for future in as_completed(futures):
            result = future.result()
            results.append(result)
    
    # Вычисляем статистику
    success_count = sum(1 for r in results if r.success)
    failed_count = len(results) - success_count
    
    durations = [r.duration_ms for r in results]
    avg_time = sum(durations) / len(durations) if durations else 0
    min_time = min(durations) if durations else 0
    max_time = max(durations) if durations else 0
    
    return BatchStats(
        iteration=iteration,
        total=len(results),
        success=success_count,
        failed=failed_count,
        avg_time_ms=avg_time,
        min_time_ms=min_time,
        max_time_ms=max_time,
        timestamp=datetime.now().strftime("%H:%M:%S")
    )


# ============================================================================
# СТАТИСТИКА: ВЫВОД В КОНСОЛЬ
# ============================================================================

def print_stats(stats: BatchStats):
    """
    Выводит статистику пачки в консоль.
    
    Формат: [TIME] Iteration #N: OK=X/Y (Z%) | AVG=Xms MIN=Yms MAX=Zms
    """
    success_rate = (stats.success / stats.total * 100) if stats.total > 0 else 0
    
    print(
        f"[{stats.timestamp}] "
        f"Iteration #{stats.iteration}: "
        f"OK={stats.success}/{stats.total} ({success_rate:.1f}%) | "
        f"AVG={stats.avg_time_ms:.0f}ms "
        f"MIN={stats.min_time_ms:.0f}ms "
        f"MAX={stats.max_time_ms:.0f}ms"
    )
    
    if stats.failed > 0:
        print(f"           ⚠ Failed: {stats.failed} requests")


# ============================================================================
# GRACEFUL SHUTDOWN
# ============================================================================

def signal_handler(signum, frame):
    """Обработчик Ctrl+C для graceful shutdown."""
    print("\n\n🛑 Прерывание по Ctrl+C. Завершение работы...")
    GlobalState.shutdown_requested = True


# ============================================================================
# MAIN LOOP
# ============================================================================

def main():
    """Основной цикл нагрузочного тестирования."""
    
    # Парсинг аргументов командной строки
    parser = argparse.ArgumentParser(
        description="Нагрузочное тестирование revsocks прокси"
    )
    parser.add_argument(
        "--proxy-host",
        default=DEFAULT_PROXY_HOST,
        help=f"Хост прокси (по умолчанию: {DEFAULT_PROXY_HOST})"
    )
    parser.add_argument(
        "--proxy-port",
        type=int,
        default=DEFAULT_PROXY_PORT,
        help=f"Порт прокси (по умолчанию: {DEFAULT_PROXY_PORT})"
    )
    parser.add_argument(
        "--proxy-user",
        default=DEFAULT_PROXY_USER,
        help=f"Пользователь прокси (по умолчанию: {DEFAULT_PROXY_USER})"
    )
    parser.add_argument(
        "--proxy-pass",
        default=DEFAULT_PROXY_PASS,
        help=f"Пароль прокси (по умолчанию: {DEFAULT_PROXY_PASS})"
    )
    parser.add_argument(
        "--target",
        default=DEFAULT_TARGET_URL,
        help=f"Целевой URL (по умолчанию: {DEFAULT_TARGET_URL})"
    )
    parser.add_argument(
        "--threads",
        type=int,
        default=DEFAULT_THREADS,
        help=f"Потоков в burst (по умолчанию: {DEFAULT_THREADS})"
    )
    parser.add_argument(
        "--interval",
        type=float,
        default=DEFAULT_INTERVAL,
        help=f"Интервал между burst в сек (по умолчанию: {DEFAULT_INTERVAL})"
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=DEFAULT_TIMEOUT,
        help=f"Таймаут запроса в сек (по умолчанию: {DEFAULT_TIMEOUT})"
    )
    parser.add_argument(
        "--max-iterations",
        type=int,
        default=None,
        help="Максимум итераций (по умолчанию: бесконечно)"
    )
    
    args = parser.parse_args()
    
    # Формируем URL прокси
    proxy_url = (
        f"socks5://{args.proxy_user}:{args.proxy_pass}@"
        f"{args.proxy_host}:{args.proxy_port}"
    )
    
    # Регистрируем обработчик Ctrl+C
    signal.signal(signal.SIGINT, signal_handler)
    
    # Выводим конфигурацию
    print("=" * 70)
    print("🚀 RevSocks Load Tester")
    print("=" * 70)
    print(f"Proxy:      {args.proxy_host}:{args.proxy_port}")
    print(f"Target:     {args.target}")
    print(f"Threads:    {args.threads}")
    print(f"Interval:   {args.interval}s")
    print(f"Timeout:    {args.timeout}s")
    print(f"Max iters:  {args.max_iterations or '∞'}")
    print("=" * 70)
    print("Нажмите Ctrl+C для остановки\n")
    
    # Основной цикл
    iteration = 0
    
    try:
        while True:
            # Проверка условий остановки
            if GlobalState.shutdown_requested:
                break
                
            if args.max_iterations and iteration >= args.max_iterations:
                print(f"\n✅ Достигнут лимит итераций: {args.max_iterations}")
                break
            
            iteration += 1
            
            # Запускаем burst
            stats = run_burst(
                iteration=iteration,
                threads=args.threads,
                proxy_url=proxy_url,
                target_url=args.target,
                timeout=args.timeout
            )
            
            # Выводим статистику
            print_stats(stats)
            
            # Ждем перед следующим burst
            if not GlobalState.shutdown_requested:
                time.sleep(args.interval)
                
    except KeyboardInterrupt:
        print("\n\n🛑 KeyboardInterrupt. Завершение...")
    
    print(f"\n📊 Всего выполнено итераций: {iteration}")
    print("Готово.")


# ============================================================================
# ENTRY POINT
# ============================================================================

if __name__ == "__main__":
    main()
