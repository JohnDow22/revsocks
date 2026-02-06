# Features Index - RevSocks

Централизованный индекс всех фич проекта с ссылками на документацию.

## ✨ Реализованные фичи

### v2.0-core - Architecture & Infrastructure

#### Project Refactoring (Separation of Concerns)
📄 [PROJECT_REFACTORING.md](PROJECT_REFACTORING.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Разделение монолитного проекта на независимые компоненты `revsocks-agent` и `revsocks-server`. Улучшение OPSEC, уменьшение размера бинарника и переход на Standard Go Layout.

---

#### E2E Testing Framework
📄 [E2E_TESTING_FRAMEWORK.md](E2E_TESTING_FRAMEWORK.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Собственный фреймворк для сквозного тестирования реальных бинарников. Позволяет проверять сложные сценарии (reconnect, failover, tls) в изолированном окружении.

---

### v2.8-stealth - Stealth Build & Failover

#### Stealth Build (Config-Driven бинарник)
📄 [STEALTH_BUILD.md](STEALTH_BUILD.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Stealth-сборка бинарника `revsocks` с захардкоженными параметрами подключения. Позволяет скрыть сервер/пароль из `ps aux` и unit-файлов, управлять всеми параметрами через `config.yaml` и использовать UPX-сжатие.

---

#### Multi-Server Failover
📄 [MULTI_SERVER_FAILOVER.md](MULTI_SERVER_FAILOVER.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Автоматическое переключение между несколькими серверами в stealth-бинарнике с учётом приоритетов, количества попыток и паузы между полными циклами.

---

### v2.9 - Extended Agent Information in Admin UI

#### Extended Agent Information in Admin UI
📄 [EXTENDED_AGENT_INFO_UI.md](EXTENDED_AGENT_INFO_UI.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Расширенная информация об агентах в Admin Console: SOCKS5 адрес, статус сессии с цветными индикаторами, версия агента, время работы сессии.

**Возможности:**
- Отображение адреса SOCKS5 прокси (IP:Port)
- Цветные индикаторы статуса (● ONLINE / ● OFFLINE)
- Версия агента (v2, v3)
- Uptime активной сессии с красивым форматированием
- Verbose режим с дополнительными полями

**Примеры:**
```bash
revsocks> agents list
# Выводит: ID, Alias, Mode, IP, SOCKS5, Status, Last Seen

revsocks> agents list -v
# Добавляет: Version, Uptime, Sleep, Jitter, First Seen
```

---

### v2.7 - Beacon Mode

#### Beacon Mode (Режим Маякования)
📄 [BEACON_MODE.md](BEACON_MODE.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Режим работы с периодическими check-in вместо постоянного TCP соединения. Агент "просыпается" через заданный интервал, получает команду от сервера (TUNNEL или SLEEP), и либо устанавливает туннель, либо снова засыпает.

**Возможности:**
- Handshake Protocol v3 (text-based)
- Persistent Agent ID (переиспользование портов)
- AgentManager с JSON persistence
- Jitter calculation (±N% от базового интервала)
- Admin HTTP API для управления агентами
- Python Admin Console (Grumble CLI)

**Примеры:**
```bash
# Запуск агента в beacon режиме
./revsocks-agent --connect server:8080 --pass test123 --beacon

# Управление через консоль
revsocks> agent sleep agent-1 3600 -j 20  # Спать ~1 час ±20%
revsocks> agent wake agent-1               # Перейти в TUNNEL режим
```

---

### v2.6 - Optimization

#### 1. Lazy TLS Certificate Caching
📄 [LAZY_TLS_CACHING.md](LAZY_TLS_CACHING.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Кеширование TLS сертификата для ускорения запусков сервера. Генерация происходит один раз, повторные запуски загружают из кеша `~/.revsocks-tls-cache/`.

**Примеры:**
- Первый запуск: 100-500ms
- Повторные: ~1ms (ускорение в 100-500 раз)

---

#### 2. Yamux Config Runtime Tuning
📄 [YAMUX_CONFIG_TUNING.md](YAMUX_CONFIG_TUNING.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Конфигурируемые параметры yamux через CLI флаги для адаптации под различные сетевые условия (спутник, мобильные сети, корпоративные сети).

**CLI флаги:**
- `-yamux-keepalive` (сек, по умолчанию 30)
- `-yamux-timeout` (сек, по умолчанию 10)

---

### v2.5 - Graceful Shutdown

#### Signal Handler & Graceful Shutdown
📄 [GRACEFUL_SHUTDOWN.md](GRACEFUL_SHUTDOWN.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Корректная обработка Ctrl+C / SIGTERM с graceful shutdown всех компонентов.

---

### v2.4 - Protocol & Synchronization

#### Protocol v2 - Length-Prefixed AgentID & ACK Handshake
📄 [PROTOCOL_V2_HANDSHAKE.md](PROTOCOL_V2_HANDSHAKE.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Новый протокол handshake с явным подтверждением (ACK) вместо time.Sleep.

---

### v2.3 - Session Lifecycle Management

#### Session Lifecycle Manager
📄 [SESSION_LIFECYCLE_MANAGER.md](SESSION_LIFECYCLE_MANAGER.md)

**Статус:** ✅ Production Ready  
**Дата:** 09.01.2026

Централизованное управление жизненным циклом сессий агентов с защитой от race conditions.

**Возможности:**
- Generation token для защиты от race
- Sticky ports через port cache
- Graceful cleanup при переподключении

---

## 📋 Быстрый поиск

| Фича | Версия | Статус | Документ |
|------|--------|--------|----------|
| Stealth Build | 2.8-stealth | ✅ | [STEALTH_BUILD.md](STEALTH_BUILD.md) |
| Multi-Server Failover | 2.3 | ✅ | [MULTI_SERVER_FAILOVER.md](MULTI_SERVER_FAILOVER.md) |
| Extended Agent Info UI | 2.9 | ✅ | [EXTENDED_AGENT_INFO_UI.md](EXTENDED_AGENT_INFO_UI.md) |
| Beacon Mode (Beaconing) | 2.7 | ✅ | [BEACON_MODE.md](BEACON_MODE.md) |
| Lazy TLS Caching | 2.6 | ✅ | [LAZY_TLS_CACHING.md](LAZY_TLS_CACHING.md) |
| Yamux Config Tuning | 2.6 | ✅ | [YAMUX_CONFIG_TUNING.md](YAMUX_CONFIG_TUNING.md) |
| Session Lifecycle Manager | 2.3 | ✅ | [SESSION_LIFECYCLE_MANAGER.md](SESSION_LIFECYCLE_MANAGER.md) |

---

**Последнее обновление:** 09.01.2026 (v2.9 - Extended Agent Info UI)  
**Версия:** 1.2
