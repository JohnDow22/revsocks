# RevSocks Refactoring: Separation of Concerns (Agent/Server)

## 1. Контекст и Цель
**Цель:** Разделить монолитный проект RevSocks (единый бинарник) на два независимых компонента: `cmd/agent` (клиент) и `cmd/server` (сервер).
**Зачем:**
1. **Stealth & Size:** Уменьшение размера бинарника агента (удаление кода сервера, DNS, листенеров).
2. **Security:** Удаление серверных сигнатур и логики из агента (Anti-Forensics).
3. **Architecture:** Приведение к стандарту `Standard Go Project Layout`.
4. **Configuration:** Упрощение флагов запуска (агент не должен знать про `-listen`).

## 2. Decision Log
| Решение | Обоснование |
|---------|-------------|
| **Standard Go Layout** | Использование `cmd/app` и `internal/pkg` для четкого разделения публичного API и внутренней логики. |
| **Shared Internal** | Вынос общей логики (`yamux`, `tls`, `version`) в `internal/transport` и `internal/common` для переиспользования без дублирования. |
| **Config Injection** | Сохранение механизма инъекции конфига через `build_stealth.sh`, но с адаптацией под `cmd/agent`. |
| **Protocol Compat** | Сохранение полной совместимости протокола (SOCKS5, Yamux, Custom Handshake). |

## 3. Матрица Зависимостей
- **Core Files:** `main.go`, `rclient.go`, `rserver.go`, `yamux_config.go`, `tlshelp.go`.
- **Build Scripts:** `Makefile`, `build_stealth.sh` (Критически важна адаптация regex-замен).
- **Tests:** `tests/e2e` (требует обновления путей к бинарникам).

## 4. Стратегия Тестирования
- **Unit Testing:**
    - Проверка изоляции пакетов `internal`.
    - Сохранение существующих тестов из `*_test.go`.
- **E2E Testing:**
    - Использование существующего E2E framework в `tests/e2e`.
    - Адаптация `builder.go` для сборки двух разных бинарников вместо одного.
    - Сценарии: Connection, Reconnection, SOCKS5 Auth, Failover.

## 5. Global Checklist
todos:
  - id: step-1-scaffold
    content: Подготовка файловой структуры (cmd/, internal/)
    status: completed ✅
    time_estimate: 15 мин
    dependencies: []
  - id: step-2-migration
    content: Миграция кода в internal/ и cmd/
    status: completed ✅
    time_estimate: 45 мин
    dependencies: [step-1-scaffold]
  - id: step-3-build-scripts
    content: Адаптация Makefile и build_stealth.sh
    status: completed ✅
    time_estimate: 60 мин
    dependencies: [step-2-migration]
  - id: step-4-verification
    content: Сборка и E2E тестирование
    status: completed ✅
    time_estimate: 30 мин
    dependencies: [step-3-build-scripts]

## 6. ROADMAP
- [x] [01_Structure_Migration.md](./01_Structure_Migration.md) - Реструктуризация и перенос кода.
- [x] [02_Build_System_Update.md](./02_Build_System_Update.md) - Обновление скриптов сборки.
- [x] [03_Verification.md](./03_Verification.md) - Тестирование и верификация.

## 7. Результаты рефакторинга (выполнено 2026-01-09)

### Новая структура
```
revsocks/
├── cmd/
│   ├── agent/main.go      # Entry point агента (~10.8 MB)
│   └── server/main.go     # Entry point сервера (~13.4 MB)
├── internal/
│   ├── common/            # Общие утилиты (version, rand, protocol)
│   ├── transport/         # Network layer (yamux, tls)
│   ├── agent/             # Логика клиента
│   ├── server/            # Логика сервера (+ SessionManager)
│   └── dns/               # DNS туннелирование
├── main.go                # Legacy (совместимость)
├── Makefile               # Обновлен для cmd/agent, cmd/server
└── build_stealth.sh       # Обновлен v2.0
```

### Команды сборки
```bash
make agent              # → revsocks-agent
make server             # → revsocks-server
make default            # → оба бинарника
make stealth            # → stealth agent из config.yaml
```
