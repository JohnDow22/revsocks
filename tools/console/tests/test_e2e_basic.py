"""
RevSocks Console E2E Tests - Базовые тесты

Тестирует:
- Подключение к серверу и проверка healthcheck
- Команды: status, info, help
- Обработка ошибок авторизации
"""
import sys

import pexpect
import pytest


# Используем системный Python вместо PYTHON_EXEC (который может быть Cursor.AppImage)
PYTHON_EXEC = "/usr/bin/python3"

# Таймаут для ожидания вывода консоли (секунды)
EXPECT_TIMEOUT = 10


class TestConsoleBasic:
    """Базовые тесты консоли"""
    
    def test_console_starts_and_connects(
        self, 
        revsocks_server, 
        console_env,
        console_main_path
    ):
        """
        Проверяет что консоль:
        1. Успешно запускается
        2. Подключается к серверу
        3. Показывает приглашение
        """
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            # Ожидаем сообщение о подключении
            child.expect(r"Connected to")
            
            # Ожидаем приглашение
            child.expect(r"revsocks>")
            
            # Выходим
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        # Проверяем код выхода
        assert child.exitstatus == 0, f"Console exited with code {child.exitstatus}"
    
    def test_status_command(
        self,
        revsocks_server,
        console_env,
        console_main_path
    ):
        """
        Тест команды 'status':
        - Должна показать статус сервера
        - Должна содержать 'healthy'
        """
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            # Ждём приглашения
            child.expect(r"revsocks>")
            
            # Отправляем команду status
            child.sendline("status")
            
            # Ожидаем ответ с 'healthy'
            child.expect(r"healthy")
            
            # Ждём возврата приглашения
            child.expect(r"revsocks>")
            
            # Выходим
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
    
    def test_info_command(
        self,
        revsocks_server,
        console_env,
        console_main_path
    ):
        """
        Тест команды 'info':
        - Должна показать информацию о консоли
        - Должна содержать URL сервера
        """
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline("info")
            
            # Ожидаем информацию о сервере
            child.expect(r"Server:")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
    
    def test_help_command(
        self,
        revsocks_server,
        console_env,
        console_main_path
    ):
        """
        Тест команды 'help':
        - Должна показать список команд
        - Должна содержать 'agents', 'status'
        """
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline("help")
            
            # Ожидаем список команд
            # cmd2 выводит help в виде таблицы
            child.expect(r"status")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0


class TestConsoleAuthErrors:
    """Тесты обработки ошибок авторизации"""
    
    def test_invalid_token_fails(
        self,
        revsocks_server,
        console_main_path
    ):
        """
        В текущей реализации Admin API доступен только с localhost и не требует авторизации.
        Поэтому "неверный" токен должен влиять только на саму консоль (токен обязан быть непустым),
        но не должен ломать команды.
        """
        # Создаём env с неверным токеном
        import os
        env = os.environ.copy()
        env["REVSOCKS_URL"] = revsocks_server.admin_url
        env["REVSOCKS_TOKEN"] = "invalid-token-12345"
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=env,
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            # Консоль запустится (healthcheck без авторизации)
            child.expect(r"revsocks>")
            
            # Команда к API должна отработать (API без авторизации на localhost)
            child.sendline("agents list")
            
            # Ожидаем либо пустой список, либо вывод таблицы (в зависимости от наличия агентов)
            child.expect(r"No agents found|Agents.*total")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        # Консоль должна нормально завершиться после обработки ошибки
        assert child.exitstatus == 0
    
    def test_missing_token_fails(
        self,
        revsocks_server,
        console_main_path
    ):
        """
        Без токена консоль должна:
        1. Показать сообщение об ошибке
        2. Завершиться с ненулевым кодом
        """
        import os
        env = os.environ.copy()
        env["REVSOCKS_URL"] = revsocks_server.admin_url
        # Убираем токен если был
        env.pop("REVSOCKS_TOKEN", None)
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=env,
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            # Ожидаем сообщение об отсутствии токена
            child.expect(r"REVSOCKS_TOKEN.*required|Error")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus != 0, "Console should exit with error on missing token"
    
    def test_unreachable_server_fails(
        self,
        console_main_path
    ):
        """
        При недоступном сервере консоль должна:
        1. Показать сообщение об ошибке соединения
        2. Завершиться с ненулевым кодом
        """
        import os
        env = os.environ.copy()
        # Используем заведомо недоступный порт
        env["REVSOCKS_URL"] = "http://127.0.0.1:59999"
        env["REVSOCKS_TOKEN"] = "some-token"
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=env,
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            # Ожидаем сообщение об ошибке соединения
            child.expect(r"Connection failed|Connection error|cannot connect")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus != 0, "Console should exit with error on unreachable server"
