"""
RevSocks Admin Console - Agent Management Commands
"""
import argparse
import re
from rich.console import Console
from rich.table import Table
from datetime import datetime
from typing import Optional

from core.api import APIClient, RevSocksAPIError

console = Console()


def parse_interval(value: str) -> int:
    """
    Парсинг интервала времени с поддержкой суффиксов.
    
    Поддерживаемые форматы:
        - 30, 30s - секунды
        - 5m - минуты  
        - 1h - часы
        - 1h30m - комбинация часов и минут
        
    Args:
        value: Строка с интервалом (например "5m", "1h", "90s", "1h30m")
        
    Returns:
        Интервал в секундах
        
    Raises:
        argparse.ArgumentTypeError: При неверном формате
    """
    value = value.strip().lower()
    
    # Если просто число - считаем секундами
    if value.isdigit():
        return int(value)
    
    # Комбинированный формат: 1h30m, 2h15m
    combined_pattern = r'^(\d+)h(\d+)m$'
    match = re.match(combined_pattern, value)
    if match:
        hours = int(match.group(1))
        minutes = int(match.group(2))
        return hours * 3600 + minutes * 60
    
    # Одиночный суффикс: 30s, 5m, 1h
    single_pattern = r'^(\d+)([smh])$'
    match = re.match(single_pattern, value)
    if match:
        num = int(match.group(1))
        unit = match.group(2)
        
        if unit == 's':
            return num
        elif unit == 'm':
            return num * 60
        elif unit == 'h':
            return num * 3600
    
    raise argparse.ArgumentTypeError(
        f"Неверный формат интервала: '{value}'. "
        f"Используйте: 30s, 5m, 1h, 1h30m"
    )


class AgentCommands:
    """Команды для управления агентами"""
    
    def __init__(self, api_client: APIClient):
        self.api = api_client
    
    # ========================================
    # Agents Command (с подкомандами)
    # ========================================
    
    def do_agents(self, args):
        """Управление агентами: agents list"""
        # Создаём парсер для подкоманд
        parser = argparse.ArgumentParser(
            prog='agents',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            description='Просмотр списка агентов',
            epilog='''
Примеры:
  agents list       # Краткий список всех агентов
  agents list -v    # Подробная информация (версия, uptime, sleep settings)
'''
        )
        subparsers = parser.add_subparsers(dest='subcommand', help='Подкоманды')
        
        # agents list
        list_parser = subparsers.add_parser(
            'list', 
            help='Список всех зарегистрированных агентов',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Колонки:
  ID            - Уникальный идентификатор агента (hostname)
  Alias         - Пользовательское имя (если задано)
  Mode          - TUNNEL (постоянное) или SLEEP (периодическое)
  IP            - IP-адрес агента
  SOCKS5        - Адрес SOCKS5 прокси для этого агента
  Status        - ONLINE/OFFLINE (таймаут 2 мин для TUNNEL, интервал*2 для SLEEP)
  First Connect - Когда агент впервые подключился
  Last Seen     - Когда агент был на связи последний раз

С флагом -v дополнительно:
  Version       - Версия агента
  Uptime        - Время текущей сессии
  Sleep         - Текущий интервал сна (секунды)
  Jitter        - Случайное отклонение от интервала

Примеры:
  agents list       # Краткий вывод
  agents list -v    # Полная информация
'''
        )
        list_parser.add_argument(
            '--verbose', '-v', 
            action='store_true', 
            help='Показать расширенную информацию (версия, uptime, sleep)'
        )
        
        try:
            parsed = parser.parse_args(args.split())
        except SystemExit:
            return
        
        if parsed.subcommand == 'list':
            self._agents_list(parsed)
        else:
            parser.print_help()
    
    def _agents_list(self, args):
        """Список всех зарегистрированных агентов"""
        try:
            agents = self.api.list_agents()
            
            if not agents:
                console.print("[yellow]No agents found[/yellow]")
                return
            
            # Создаём таблицу
            table = Table(title=f"Agents ({len(agents)} total)")
            table.add_column("ID", style="cyan", no_wrap=True)
            table.add_column("Alias", style="green")
            table.add_column("Mode", style="bold magenta")
            table.add_column("IP", style="blue")
            table.add_column("SOCKS5", style="yellow", no_wrap=True)
            table.add_column("Status", style="bold")
            table.add_column("First Connect", style="dim")
            table.add_column("Last Seen", style="dim")
            
            if args.verbose:
                table.add_column("Version", style="dim")
                table.add_column("Uptime", style="dim")
                table.add_column("Sleep", style="dim")
                table.add_column("Jitter", style="dim")
            
            # Заполняем таблицу
            for agent in agents:
                agent_id = agent.get("id", "N/A")
                alias = agent.get("alias", "-")
                mode = agent.get("mode", "UNKNOWN")
                ip = agent.get("ip", "N/A")
                socks_addr = agent.get("socks_addr", "-")
                is_online = agent.get("is_online", False)
                last_seen = self._format_time(agent.get("last_seen"))
                
                # Статус: online/offline
                status = "[green]●[/green] ONLINE" if is_online else "[red]●[/red] OFFLINE"
                first_connect = self._format_time(agent.get("first_seen"))
                
                row = [agent_id, alias, mode, ip, socks_addr, status, first_connect, last_seen]
                
                if args.verbose:
                    version = agent.get("version", "-")
                    uptime = self._format_uptime(agent.get("session_uptime", 0))
                    sleep_interval = str(agent.get("sleep_interval", "-"))
                    jitter = f"{agent.get('jitter', 0)}%"
                    row.extend([version, uptime, sleep_interval, jitter])
                table.add_row(*row)
            
            console.print(table)
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    # ========================================
    # Agent Command (с подкомандами)
    # ========================================
    
    def do_agent(self, args):
        """Управление агентом: agent sleep|wake|rename|delete <agent_id>"""
        # Создаём парсер для подкоманд
        parser = argparse.ArgumentParser(
            prog='agent',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            description='Управление отдельным агентом',
            epilog='''
Примеры:
  agent sleep notebook 5m          # Спать 5 минут
  agent sleep notebook 1h -j 20    # Спать 1 час с jitter 20%
  agent sleep notebook 1h30m       # Спать 1.5 часа
  agent wake notebook              # Вернуть в постоянный режим
  agent rename notebook prod-dc1   # Задать понятное имя
  agent delete notebook -f         # Удалить без подтверждения
'''
        )
        subparsers = parser.add_subparsers(dest='subcommand', help='Подкоманды')
        
        # agent sleep
        sleep_parser = subparsers.add_parser(
            'sleep', 
            help='Перевести агента в SLEEP режим (периодические подключения)',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Форматы интервала:
  30, 30s    - секунды (agent sleep myhost 30s)
  5m         - минуты  (agent sleep myhost 5m)
  1h         - часы    (agent sleep myhost 1h)
  1h30m      - комбинация (agent sleep myhost 1h30m)

Примеры:
  agent sleep notebook 5m          # Спать 5 минут (300 сек)
  agent sleep notebook 1h          # Спать 1 час (3600 сек)
  agent sleep notebook 30m -j 25   # Спать 30 мин, jitter 25%
  agent sleep notebook 1h30m       # Спать 1.5 часа (5400 сек)
'''
        )
        sleep_parser.add_argument('agent_id', help='ID или alias агента')
        sleep_parser.add_argument(
            'interval', 
            type=parse_interval, 
            help='Интервал сна (30s, 5m, 1h, 1h30m)'
        )
        sleep_parser.add_argument(
            '--jitter', '-j', 
            type=int, 
            default=10,
            metavar='%',
            help='Случайное отклонение от интервала в %% (default: 10)'
        )
        
        # agent wake
        wake_parser = subparsers.add_parser(
            'wake', 
            help='Перевести агента в TUNNEL режим (постоянное соединение)',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Примеры:
  agent wake notebook     # Агент установит постоянное соединение
  agent wake prod-dc1     # Можно использовать alias
'''
        )
        wake_parser.add_argument('agent_id', help='ID или alias агента')
        
        # agent rename
        rename_parser = subparsers.add_parser(
            'rename', 
            help='Установить человекочитаемый алиас для агента',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Примеры:
  agent rename f4d3b2a1 prod-dc1      # Задать имя по ID
  agent rename notebook web-server    # Переименовать
'''
        )
        rename_parser.add_argument('agent_id', help='Текущий ID или alias агента')
        rename_parser.add_argument('alias', help='Новый алиас (без пробелов)')
        
        # agent delete
        delete_parser = subparsers.add_parser(
            'delete', 
            help='Удалить агента из базы данных',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Примеры:
  agent delete notebook       # С подтверждением
  agent delete notebook -f    # Без подтверждения
'''
        )
        delete_parser.add_argument('agent_id', help='ID или alias агента')
        delete_parser.add_argument(
            '--force', '-f', 
            action='store_true', 
            help='Удалить без подтверждения'
        )
        
        try:
            parsed = parser.parse_args(args.split())
        except SystemExit:
            return
        
        if parsed.subcommand == 'sleep':
            self._agent_sleep(parsed)
        elif parsed.subcommand == 'wake':
            self._agent_wake(parsed)
        elif parsed.subcommand == 'rename':
            self._agent_rename(parsed)
        elif parsed.subcommand == 'delete':
            self._agent_delete(parsed)
        else:
            parser.print_help()
    
    def _agent_sleep(self, args):
        """Перевести агента в SLEEP режим"""
        try:
            result = self.api.update_agent(
                agent_id=args.agent_id,
                mode="SLEEP",
                sleep_interval=args.interval,
                jitter=args.jitter,
            )
            
            # Форматируем интервал для вывода
            interval_str = self._format_interval(args.interval)
            
            console.print(f"[green]✓[/green] Agent {args.agent_id} set to SLEEP mode:")
            console.print(f"  Interval: {interval_str} ({args.interval}s)")
            console.print(f"  Jitter: {args.jitter}%")
            console.print(f"[dim]Agent will receive this config on next check-in[/dim]")
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    def _agent_wake(self, args):
        """Перевести агента в TUNNEL режим (постоянное соединение)"""
        try:
            result = self.api.update_agent(
                agent_id=args.agent_id,
                mode="TUNNEL",
            )
            
            console.print(f"[green]✓[/green] Agent {args.agent_id} set to TUNNEL mode")
            console.print(f"[dim]Agent will establish persistent connection on next check-in[/dim]")
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    def _agent_rename(self, args):
        """Установить человекочитаемый алиас для агента"""
        try:
            result = self.api.update_agent(
                agent_id=args.agent_id,
                alias=args.alias,
            )
            
            console.print(f"[green]✓[/green] Agent {args.agent_id} renamed to '{args.alias}'")
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    def _agent_delete(self, args):
        """Удалить агента из базы данных"""
        if not args.force:
            console.print(f"[yellow]⚠ This will remove agent {args.agent_id} from database[/yellow]")
            confirmation = input("Continue? [y/N]: ")
            if confirmation.lower() != "y":
                console.print("[dim]Cancelled[/dim]")
                return
        
        try:
            result = self.api.delete_agent(args.agent_id)
            console.print(f"[green]✓[/green] Agent {args.agent_id} deleted")
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    # ========================================
    # Session Command
    # ========================================
    
    def do_session(self, args):
        """Управление сессиями: session kill <agent_id>"""
        # Создаём парсер для подкоманд
        parser = argparse.ArgumentParser(
            prog='session',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            description='Управление активными yamux сессиями',
            epilog='''
Примеры:
  session kill notebook    # Принудительно закрыть сессию агента
'''
        )
        subparsers = parser.add_subparsers(dest='subcommand', help='Подкоманды')
        
        # session kill
        kill_parser = subparsers.add_parser(
            'kill', 
            help='Принудительно закрыть yamux сессию агента',
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog='''
Закрывает активное соединение с агентом. Агент переподключится
в соответствии со своим режимом:
  - TUNNEL: немедленно
  - SLEEP: через интервал сна ± jitter

Используйте когда:
  - Сессия "зависла"
  - Нужно принудительно применить новые настройки
  - Тестирование переподключения

Примеры:
  session kill notebook
  session kill prod-dc1
'''
        )
        kill_parser.add_argument('agent_id', help='ID или alias агента')
        
        try:
            parsed = parser.parse_args(args.split())
        except SystemExit:
            return
        
        if parsed.subcommand == 'kill':
            self._session_kill(parsed)
        else:
            parser.print_help()
    
    def _session_kill(self, args):
        """Убить активную сессию агента (закрыть yamux)"""
        try:
            result = self.api.kill_session(args.agent_id)
            console.print(f"[green]✓[/green] Session killed for agent {args.agent_id}")
            console.print(f"[dim]Agent will reconnect based on its mode (TUNNEL/SLEEP)[/dim]")
            
        except RevSocksAPIError as e:
            console.print(f"[red]Error: {e}[/red]")
    
    # ========================================
    # Helpers
    # ========================================
    
    def _format_time(self, time_str: Optional[str]) -> str:
        """Форматировать время в читаемый вид"""
        if not time_str:
            return "N/A"
        
        try:
            dt = datetime.fromisoformat(time_str.replace("Z", "+00:00"))
            now = datetime.now(dt.tzinfo)
            delta = now - dt
            
            # Показываем относительное время для недавних событий
            if delta.total_seconds() < 60:
                return f"{int(delta.total_seconds())}s ago"
            elif delta.total_seconds() < 3600:
                return f"{int(delta.total_seconds() / 60)}m ago"
            elif delta.total_seconds() < 86400:
                hours = int(delta.total_seconds() / 3600)
                minutes = int((delta.total_seconds() % 3600) / 60)
                return f"{hours}h {minutes}m ago"
            else:
                # Для старых дат - читаемый формат без миллисекунд и таймзоны
                return dt.strftime("%d.%m.%Y %H:%M")
        except:
            # Fallback: попытка убрать миллисекунды и таймзону вручную
            try:
                # Обрезаем миллисекунды и таймзону: "2026-01-10T11:06:11.402841243+07:00"
                if "." in time_str:
                    time_str = time_str.split(".")[0]  # "2026-01-10T11:06:11"
                dt = datetime.fromisoformat(time_str)
                return dt.strftime("%d.%m.%Y %H:%M")
            except:
                return time_str
    
    def _format_uptime(self, seconds: int) -> str:
        """Форматировать uptime в читаемый вид"""
        if seconds == 0:
            return "-"
        
        if seconds < 60:
            return f"{seconds}s"
        elif seconds < 3600:
            minutes = seconds // 60
            return f"{minutes}m"
        elif seconds < 86400:
            hours = seconds // 3600
            minutes = (seconds % 3600) // 60
            return f"{hours}h {minutes}m"
        else:
            days = seconds // 86400
            hours = (seconds % 86400) // 3600
            return f"{days}d {hours}h"
    
    def _format_interval(self, seconds: int) -> str:
        """Форматировать интервал в читаемый вид (для вывода)"""
        if seconds < 60:
            return f"{seconds}s"
        elif seconds < 3600:
            minutes = seconds // 60
            remaining_seconds = seconds % 60
            if remaining_seconds:
                return f"{minutes}m{remaining_seconds}s"
            return f"{minutes}m"
        else:
            hours = seconds // 3600
            remaining_minutes = (seconds % 3600) // 60
            if remaining_minutes:
                return f"{hours}h{remaining_minutes}m"
            return f"{hours}h"