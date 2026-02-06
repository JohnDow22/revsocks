"""
RevSocks Admin Console - Configuration
"""
import os
from pathlib import Path

# Путь к конфигурационному файлу (относительно скрипта)
CONFIG_DIR = Path(__file__).parent
DEFAULT_CONFIG_PATH = CONFIG_DIR / "config.yaml"

# Дефолтные значения конфигурации
DEFAULT_CONFIG = {
    "server": {
        "url": "http://127.0.0.1:8081",
        "token": "",  # Токен не используется (API без авторизации)
    },
    "console": {
        "prompt": "revsocks> ",
        "history_file": str(Path.home() / ".revsocks_console_history"),
    }
}
