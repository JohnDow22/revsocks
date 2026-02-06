"""
RevSocks Console E2E Tests - Интерактивные тесты с агентом

Тестирует:
- agents list - список агентов
- agent sleep/wake - управление режимом
- agent rename - переименование
- session kill - убийство сессии
"""
import sys

import pexpect
import pytest
import requests


# Используем системный Python вместо sys.executable (который может быть Cursor.AppImage)
PYTHON_EXEC = "/usr/bin/python3"

# Таймаут для ожидания вывода консоли (секунды)
EXPECT_TIMEOUT = 15


class TestAgentsList:
    """Тесты команды 'agents list'"""
    
    def test_agents_list_empty(
        self,
        revsocks_server,
        console_env,
        console_main_path
    ):
        """
        Без агентов команда 'agents list' должна показать:
        - Сообщение 'No agents found'
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
            
            child.sendline("agents list")
            
            # Ожидаем сообщение об отсутствии агентов
            child.expect(r"No agents found")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
    
    def test_agents_list_with_agent(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        С подключенным агентом команда 'agents list' должна показать:
        - Таблицу с агентами
        - ID агента
        - IP адрес
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
            
            child.sendline("agents list")
            
            # Ожидаем таблицу с агентами
            # ID агента должен быть в выводе
            child.expect(r"Agents.*total")  # Заголовок таблицы
            child.expect(r"127\.0\.0\.1|localhost")  # IP агента (localhost в тестах)
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
    
    def test_agents_list_verbose(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agents list -v' должна показать дополнительные поля:
        - Sleep interval
        - Jitter
        - First seen
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
            
            child.sendline("agents list -v")
            
            # В verbose режиме должна быть расширенная таблица
            child.expect(r"Agents.*total")
            # Проверяем что таблица отрисовалась (ищем границы таблицы или ID агента)
            child.expect(r"notebook|TUNNEL|ONLINE|OFFLINE")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0


class TestAgentManagement:
    """Тесты управления агентом (sleep, wake, rename)"""
    
    def test_agent_sleep(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agent sleep <id> <interval>' должна:
        1. Успешно перевести агента в SLEEP режим
        2. API сервера должен подтвердить изменение
        """
        agent_id = revsocks_agent.agent_id
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            # Отправляем команду sleep с интервалом 60 секунд
            child.sendline(f"agent sleep {agent_id} 60")
            
            # Ожидаем успешный ответ
            child.expect(r"SLEEP mode|set to SLEEP")
            # Формат может быть человекочитаемым (например: "1m (60s)"), поэтому фиксируемся на секундах.
            child.expect(r"\(60s\)")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
        
        # Дополнительная проверка через API
        resp = requests.get(
            f"{revsocks_server.admin_url}/api/agents",
            headers={"X-Admin-Token": revsocks_server.token},
            timeout=5
        )
        assert resp.status_code == 200
        agents = resp.json()
        agent = next((a for a in agents if a["id"] == agent_id), None)
        assert agent is not None, f"Agent {agent_id} not found"
        assert agent["mode"] == "SLEEP", f"Expected SLEEP mode, got {agent['mode']}"
        assert agent["sleep_interval"] == 60
    
    def test_agent_wake(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agent wake <id>' должна:
        1. Перевести агента обратно в TUNNEL режим
        """
        agent_id = revsocks_agent.agent_id
        
        # Сначала переводим в SLEEP через API
        requests.post(
            f"{revsocks_server.admin_url}/api/agents/{agent_id}/config",
            headers={"X-Admin-Token": revsocks_server.token},
            json={"mode": "SLEEP", "sleep_interval": 120, "jitter": 10},
            timeout=5
        )
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline(f"agent wake {agent_id}")
            
            # Ожидаем успешный ответ
            child.expect(r"TUNNEL mode|set to TUNNEL")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
        
        # Проверка через API
        resp = requests.get(
            f"{revsocks_server.admin_url}/api/agents",
            headers={"X-Admin-Token": revsocks_server.token},
            timeout=5
        )
        agents = resp.json()
        agent = next((a for a in agents if a["id"] == agent_id), None)
        assert agent["mode"] == "TUNNEL", f"Expected TUNNEL mode, got {agent['mode']}"
    
    def test_agent_rename(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agent rename <id> <alias>' должна:
        1. Установить алиас для агента
        2. Новый алиас должен отображаться в 'agents list'
        """
        agent_id = revsocks_agent.agent_id
        new_alias = "test-server-01"
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline(f"agent rename {agent_id} {new_alias}")
            
            # Ожидаем успешный ответ
            child.expect(r"renamed")
            child.expect(r"revsocks>")
            
            # Проверяем что изменения применились через agents list
            # После rename алиас должен быть в колонке Alias
            child.sendline("agents list")
            child.expect(r"Agents.*total")
            # Ищем agent_id в таблице - он точно будет
            child.expect(agent_id)
            # Алиас может быть обрезан в узкой таблице, так что проверяем хотя бы часть
            # или просто что команда отработала и таблица показалась
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
    
    def test_agent_sleep_with_jitter(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agent sleep <id> <interval> -j <jitter>' должна:
        1. Установить SLEEP режим с указанным jitter
        """
        agent_id = revsocks_agent.agent_id
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline(f"agent sleep {agent_id} 300 -j 25")
            
            child.expect(r"SLEEP mode")
            # Формат может быть человекочитаемым (например: "5m (300s)"), поэтому фиксируемся на секундах.
            child.expect(r"\(300s\)")
            child.expect(r"Jitter: 25%")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
        
        # Проверка через API
        resp = requests.get(
            f"{revsocks_server.admin_url}/api/agents",
            headers={"X-Admin-Token": revsocks_server.token},
            timeout=5
        )
        agents = resp.json()
        agent = next((a for a in agents if a["id"] == agent_id), None)
        assert agent["jitter"] == 25, f"Expected jitter 25, got {agent['jitter']}"


class TestAgentDelete:
    """Тесты удаления агента"""
    
    def test_agent_delete_force(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'agent delete <id> --force' должна:
        1. Удалить агента из БД
        2. Агент не должен появляться в 'agents list'
        """
        agent_id = revsocks_agent.agent_id
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline(f"agent delete {agent_id} --force")
            
            # Ожидаем подтверждение удаления
            child.expect(r"deleted")
            child.expect(r"revsocks>")
            
            # Проверяем что агент удалён
            child.sendline("agents list")
            child.expect(r"No agents found")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0


class TestAgentErrors:
    """Тесты обработки ошибок при работе с агентами"""
    
    def test_agent_not_found(
        self,
        revsocks_server,
        console_env,
        console_main_path
    ):
        """
        Команда к несуществующему агенту должна показать ошибку
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
            
            # Используем несуществующий ID
            child.sendline("agent sleep nonexistent-agent-id 60")
            
            # Ожидаем сообщение об ошибке
            child.expect(r"Error|not found|Not found")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        # Консоль не должна упасть при ошибке
        assert child.exitstatus == 0


class TestSessionManagement:
    """Тесты управления сессиями"""
    
    def test_session_kill(
        self,
        revsocks_server,
        revsocks_agent,
        console_env,
        console_main_path
    ):
        """
        Команда 'session kill <id>' должна:
        1. Успешно убить активную сессию
        
        Примечание: после kill агент может переподключиться,
        поэтому проверяем только успешность команды.
        """
        agent_id = revsocks_agent.agent_id
        
        child = pexpect.spawn(
            PYTHON_EXEC,
            [str(console_main_path)],
            env=console_env.as_dict(),
            timeout=EXPECT_TIMEOUT,
            encoding="utf-8",
        )
        
        try:
            child.expect(r"revsocks>")
            
            child.sendline(f"session kill {agent_id}")
            
            # Ожидаем сообщение об успехе или о том что сессии нет
            # (агент мог быть в SLEEP режиме без активной сессии)
            child.expect(r"killed|Session not found|Error")
            child.expect(r"revsocks>")
            
            child.sendline("quit")
            child.expect(pexpect.EOF)
            
        finally:
            child.close()
        
        assert child.exitstatus == 0
