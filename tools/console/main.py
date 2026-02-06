#!/usr/bin/env python3
"""
RevSocks Admin Console - Main Entry Point
cmd2-based CLI для управления RevSocks агентами
"""
import sys
import os
import logging
from pathlib import Path

# Добавляем текущую директорию в PYTHONPATH для импортов
sys.path.insert(0, str(Path(__file__).parent))

import cmd2
from rich.console import Console

from config import DEFAULT_CONFIG
from core.api import APIClient, RevSocksAPIError
from commands.agents import AgentCommands

console = Console()


def load_config():
    """Загружает конфигурацию (упрощённая версия без yaml)"""
    # API работает без авторизации (только localhost)
    url = os.environ.get("REVSOCKS_URL", "http://127.0.0.1:8081")
    
    return {
        "server": {"url": url, "token": ""},  # Токен не используется
        "console": DEFAULT_CONFIG["console"],
    }


class RevSocksConsole(cmd2.Cmd):
    """RevSocks Admin Console на базе cmd2"""
    
    def __init__(self, config, api_client):
        """Инициализация консоли"""
        super().__init__(
            persistent_history_file=config["console"]["history_file"],
            startup_script=None,
        )
        
        self.config = config
        self.api = api_client
        self.prompt = config["console"]["prompt"]
        self.intro = "RevSocks Admin Console. Type 'help' for commands."
        
        # Скрываем встроенные cmd2 команды которые не нужны для RevSocks
        # Они всё ещё работают если пользователь знает про них, но не засоряют help
        self.hidden_commands.extend([
            "alias", "edit", "macro", "run_pyscript", "run_script",
            "set", "shell", "shortcuts", "history",
        ])
        
        # Регистрируем команды агентов
        self.agent_commands = AgentCommands(api_client)
        
        # Добавляем методы команд в текущий класс
        self.do_agents = self.agent_commands.do_agents
        self.do_agent = self.agent_commands.do_agent
        self.do_session = self.agent_commands.do_session
    
    def do_status(self, _):
        """Статус подключения к серверу"""
        try:
            health = self.api.health_check()
            console.print(f"[green]✓[/green] Server: {self.config['server']['url']}")
            console.print(f"  Status: [green]healthy[/green]")
            console.print(f"  Time: {health.get('time', 'N/A')}")
        except RevSocksAPIError as e:
            console.print(f"[red]✗[/red] Server unhealthy: {e}")
    
    def do_info(self, _):
        """Информация о консоли"""
        console.print("[bold]RevSocks Admin Console[/bold]")
        console.print(f"Server: {self.config['server']['url']}")
        console.print(f"Commands: agents list, agent sleep, agent wake, etc.")
        console.print("Type 'help' for full command list")


def main():
    """Главная функция"""
    
    # Настройка логирования (минимальный уровень)
    logging.basicConfig(
        level=logging.WARNING,
        format="%(asctime)s [%(levelname)s] %(message)s",
    )
    
    # Загружаем конфигурацию
    config = load_config()
    
    # Инициализируем API клиент
    api_client = APIClient(
        base_url=config["server"]["url"],
        token=config["server"]["token"],
    )
    
    # Проверяем соединение
    try:
        health = api_client.health_check()
        console.print(f"[green]✓[/green] Connected to {config['server']['url']}")
    except RevSocksAPIError as e:
        console.print(f"[red]Connection failed: {e}[/red]")
        console.print("[dim]Make sure the server is running and token is correct[/dim]")
        sys.exit(1)
    
    # Создаём и запускаем консоль
    app = RevSocksConsole(config, api_client)
    
    try:
        app.cmdloop()
    except KeyboardInterrupt:
        console.print("\n[dim]Bye![/dim]")
    except Exception as e:
        console.print(f"[red]Fatal error: {e}[/red]")
        sys.exit(1)


if __name__ == "__main__":
    main()
