"""
RevSocks Admin Console - API Client Wrapper
"""
import requests
from typing import Dict, List, Optional, Any
import json
import logging

logger = logging.getLogger(__name__)


class RevSocksAPIError(Exception):
    """Базовая ошибка API"""
    pass


class APIClient:
    """Wrapper для HTTP API RevSocks"""
    
    def __init__(self, base_url: str, token: str = "", timeout: int = 10):
        """
        Инициализация API клиента
        
        Args:
            base_url: Базовый URL сервера (например http://127.0.0.1:8081)
            token: Не используется (API без авторизации, только localhost)
            timeout: Таймаут запросов в секундах
        """
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.timeout = timeout
        self.session = requests.Session()
        self.session.headers.update({
            "Content-Type": "application/json",
        })
    
    def _request(self, method: str, endpoint: str, data: Optional[Dict] = None) -> Any:
        """
        Универсальный метод для HTTP запросов
        
        Args:
            method: HTTP метод (GET, POST, DELETE)
            endpoint: API endpoint (например /api/agents)
            data: JSON данные для отправки (для POST)
            
        Returns:
            Распарсенный JSON ответ
            
        Raises:
            RevSocksAPIError: При ошибке HTTP или API
        """
        url = f"{self.base_url}{endpoint}"
        
        try:
            if method == "GET":
                response = self.session.get(url, timeout=self.timeout)
            elif method == "POST":
                response = self.session.post(url, json=data, timeout=self.timeout)
            elif method == "DELETE":
                response = self.session.delete(url, timeout=self.timeout)
            else:
                raise RevSocksAPIError(f"Unsupported HTTP method: {method}")
            
            # Проверяем статус код
            if response.status_code == 404:
                raise RevSocksAPIError("Not found")
            elif response.status_code >= 400:
                try:
                    error_data = response.json()
                    error_msg = error_data.get("error", response.text)
                except:
                    error_msg = response.text
                raise RevSocksAPIError(f"API error ({response.status_code}): {error_msg}")
            
            # Парсим успешный ответ
            return response.json()
            
        except requests.exceptions.ConnectionError:
            raise RevSocksAPIError(f"Connection error: cannot connect to {self.base_url}")
        except requests.exceptions.Timeout:
            raise RevSocksAPIError(f"Request timeout after {self.timeout}s")
        except requests.exceptions.RequestException as e:
            raise RevSocksAPIError(f"Request failed: {str(e)}")
    
    # ========================================
    # Agent Management Endpoints
    # ========================================
    
    def list_agents(self) -> List[Dict]:
        """
        Получить список всех агентов
        
        Returns:
            Список агентов с их конфигурацией
        """
        return self._request("GET", "/api/agents")
    
    def update_agent(
        self,
        agent_id: str,
        mode: Optional[str] = None,
        sleep_interval: Optional[int] = None,
        jitter: Optional[int] = None,
        alias: Optional[str] = None,
    ) -> Dict:
        """
        Обновить конфигурацию агента
        
        Args:
            agent_id: ID агента
            mode: Режим работы ("TUNNEL" или "SLEEP")
            sleep_interval: Интервал сна в секундах (для SLEEP режима)
            jitter: Jitter в процентах (0-100)
            alias: Человекочитаемый алиас
            
        Returns:
            Обновлённая конфигурация агента
        """
        data = {}
        if mode is not None:
            data["mode"] = mode
        if sleep_interval is not None:
            data["sleep_interval"] = sleep_interval
        if jitter is not None:
            data["jitter"] = jitter
        if alias is not None:
            data["alias"] = alias
        
        if not data:
            raise RevSocksAPIError("At least one parameter must be specified")
        
        return self._request("POST", f"/api/agents/{agent_id}/config", data=data)
    
    def delete_agent(self, agent_id: str) -> Dict:
        """
        Удалить агента из базы
        
        Args:
            agent_id: ID агента
            
        Returns:
            Статус удаления
        """
        return self._request("DELETE", f"/api/agents/{agent_id}")
    
    def kill_session(self, agent_id: str) -> Dict:
        """
        Убить активную сессию агента (закрыть yamux)
        
        Args:
            agent_id: ID агента
            
        Returns:
            Статус операции
        """
        return self._request("DELETE", f"/api/sessions/{agent_id}")
    
    def health_check(self) -> Dict:
        """
        Проверка доступности API
        
        Returns:
            Статус сервера
        """
        return self._request("GET", "/health")
