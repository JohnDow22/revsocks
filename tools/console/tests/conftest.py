"""
RevSocks Console E2E Tests - Фикстуры для управления процессами

Управляет жизненным циклом:
- revsocks-server (запускается на динамическом порту)
- revsocks-agent (подключается к серверу)
- Переменные окружения для консоли
"""
import os
import socket
import subprocess
import tempfile
import time
import random
import string
from pathlib import Path
from dataclasses import dataclass
from typing import Optional, Generator
import requests
import pytest


# ========================================
# Конфигурация путей
# ========================================

# Базовая директория проекта (revsocks/)
REVSOCKS_ROOT = Path(__file__).parent.parent.parent.parent
CONSOLE_DIR = Path(__file__).parent.parent

# Пути к бинарникам (можно переопределить через ENV)
DEFAULT_SERVER_BIN = REVSOCKS_ROOT / "revsocks-server"
# revsocks-agent-test - обычная сборка с CLI флагами (не baked)
# revsocks-agent_v3 - stealth сборка с захардкоженным конфигом
DEFAULT_AGENT_BIN = REVSOCKS_ROOT / "revsocks-agent-test"


# ========================================
# Датаклассы для конфигурации
# ========================================

@dataclass
class ServerConfig:
    """Конфигурация запущенного сервера"""
    process: subprocess.Popen
    listen_port: int
    admin_port: int
    socks_port: int
    token: str
    password: str
    agentdb_path: str  # Путь к временному файлу БД агентов
    
    @property
    def listen_url(self) -> str:
        return f"127.0.0.1:{self.listen_port}"
    
    @property
    def admin_url(self) -> str:
        return f"http://127.0.0.1:{self.admin_port}"
    
    @property
    def socks_addr(self) -> str:
        return f"127.0.0.1:{self.socks_port}"


@dataclass 
class AgentConfig:
    """Конфигурация запущенного агента"""
    process: subprocess.Popen
    agent_id: str  # Будет заполнено после регистрации


@dataclass
class ConsoleEnv:
    """Переменные окружения для запуска консоли"""
    url: str
    token: str
    
    def as_dict(self) -> dict:
        """Возвращает dict для передачи в subprocess/pexpect"""
        env = os.environ.copy()
        env["REVSOCKS_URL"] = self.url
        env["REVSOCKS_TOKEN"] = self.token
        return env


# ========================================
# Утилиты
# ========================================

def get_free_port() -> int:
    """Получить свободный TCP порт"""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def generate_password(length: int = 16) -> str:
    """Генерация случайного пароля"""
    chars = string.ascii_letters + string.digits
    return "".join(random.choices(chars, k=length))


def wait_for_health(url: str, timeout: float = 10.0, interval: float = 0.3) -> bool:
    """
    Ожидание healthcheck сервера
    
    Args:
        url: URL сервера (без /health)
        timeout: Максимальное время ожидания
        interval: Интервал между проверками
        
    Returns:
        True если сервер доступен, False по таймауту
    """
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            resp = requests.get(f"{url}/health", timeout=2)
            if resp.status_code == 200:
                return True
        except requests.RequestException:
            pass
        time.sleep(interval)
    return False


def wait_for_agent_registration(
    admin_url: str, 
    token: str,
    timeout: float = 15.0, 
    interval: float = 0.5
) -> Optional[str]:
    """
    Ожидание регистрации агента на сервере
    
    Returns:
        ID агента или None по таймауту
    """
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            resp = requests.get(
                f"{admin_url}/api/agents",
                headers={"X-Admin-Token": token},
                timeout=2
            )
            if resp.status_code == 200:
                agents = resp.json()
                if agents:
                    # Возвращаем ID первого агента
                    return agents[0].get("id")
        except requests.RequestException:
            pass
        time.sleep(interval)
    return None


# ========================================
# Фикстуры
# ========================================

@pytest.fixture(scope="session")
def revsocks_binaries():
    """
    Проверяет наличие бинарников и возвращает пути к ним.
    Можно переопределить через переменные окружения:
    - TEST_SERVER_BIN
    - TEST_AGENT_BIN
    """
    server_bin = Path(os.environ.get("TEST_SERVER_BIN", DEFAULT_SERVER_BIN))
    agent_bin = Path(os.environ.get("TEST_AGENT_BIN", DEFAULT_AGENT_BIN))
    
    if not server_bin.exists():
        pytest.skip(f"Server binary not found: {server_bin}")
    if not agent_bin.exists():
        pytest.skip(f"Agent binary not found: {agent_bin}")
    
    # Проверяем права на выполнение
    if not os.access(server_bin, os.X_OK):
        pytest.skip(f"Server binary not executable: {server_bin}")
    if not os.access(agent_bin, os.X_OK):
        pytest.skip(f"Agent binary not executable: {agent_bin}")
    
    return {"server": server_bin, "agent": agent_bin}


@pytest.fixture
def revsocks_server(revsocks_binaries) -> Generator[ServerConfig, None, None]:
    """
    Запускает сервер RevSocks с Admin API на динамических портах.
    
    Использует yield pattern для гарантированной остановки после теста.
    Каждый тест получает изолированную БД агентов.
    """
    # Получаем свободные порты
    listen_port = get_free_port()
    admin_port = get_free_port()
    socks_port = get_free_port()
    
    password = generate_password()
    token = generate_password(32)
    
    server_bin = revsocks_binaries["server"]
    
    # Создаём временный файл для БД агентов (изоляция тестов)
    agentdb_fd, agentdb_path = tempfile.mkstemp(suffix=".json", prefix="revsocks_test_agents_")
    os.close(agentdb_fd)  # Закрываем дескриптор, файл будет использован сервером
    
    # Формируем команду запуска
    cmd = [
        str(server_bin),
        "-listen", f":{listen_port}",
        "-socks", f"127.0.0.1:{socks_port}",
        "-pass", password,
        "-admin-api",
        "-admin-port", f":{admin_port}",
        "-agentdb", agentdb_path,  # Изолированная БД для теста
        # Без TLS для тестов (упрощает)
    ]
    
    # Запускаем сервер
    process = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    
    # Создаём конфиг
    config = ServerConfig(
        process=process,
        listen_port=listen_port,
        admin_port=admin_port,
        socks_port=socks_port,
        token=token,
        password=password,
        agentdb_path=agentdb_path,
    )
    
    # Ожидаем готовности сервера
    if not wait_for_health(config.admin_url, timeout=10.0):
        # Сервер не стартовал - читаем stderr для диагностики
        process.terminate()
        process.wait(timeout=5)
        stderr = process.stderr.read() if process.stderr else "N/A"
        # Удаляем временный файл
        try:
            os.unlink(agentdb_path)
        except OSError:
            pass
        pytest.fail(f"Server failed to start. stderr:\n{stderr}")
    
    yield config
    
    # Cleanup: убиваем сервер
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
    
    # Удаляем временный файл БД
    try:
        os.unlink(agentdb_path)
    except OSError:
        pass


@pytest.fixture
def revsocks_agent(revsocks_server, revsocks_binaries) -> Generator[AgentConfig, None, None]:
    """
    Запускает агента RevSocks, подключает к серверу.
    
    Ожидает регистрации агента через Admin API.
    """
    agent_bin = revsocks_binaries["agent"]
    
    # Формируем команду запуска агента
    # Примечание: в текущем агенте v3/beacon режим используется по умолчанию.
    # Флага -beacon в CLI нет, поэтому не используем его в тестах.
    cmd = [
        str(agent_bin),
        "-connect", revsocks_server.listen_url,
        "-pass", revsocks_server.password,
        "-q",  # Quiet mode
        # Без TLS так как сервер без TLS
    ]
    
    # Запускаем агента
    process = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    
    # Ожидаем регистрации агента на сервере
    agent_id = wait_for_agent_registration(
        revsocks_server.admin_url,
        revsocks_server.token,
        timeout=15.0,
    )
    
    if not agent_id:
        # Агент не зарегистрировался
        process.terminate()
        process.wait(timeout=5)
        stderr = process.stderr.read() if process.stderr else "N/A"
        pytest.fail(f"Agent failed to register. stderr:\n{stderr}")
    
    config = AgentConfig(
        process=process,
        agent_id=agent_id,
    )
    
    yield config
    
    # Cleanup: убиваем агента
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()


@pytest.fixture
def console_env(revsocks_server) -> ConsoleEnv:
    """
    Подготавливает переменные окружения для запуска консоли.
    """
    return ConsoleEnv(
        url=revsocks_server.admin_url,
        token=revsocks_server.token,
    )


@pytest.fixture
def console_main_path() -> Path:
    """Путь к main.py консоли"""
    return CONSOLE_DIR / "main.py"
