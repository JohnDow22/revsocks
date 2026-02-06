# 00_PLAN_INDEX.md

## 🎯 Context & Goal
Создание полнофункциональной E2E (End-to-End) системы тестирования для `RevSocks`.
Текущие юнит-тесты проверяют отдельные функции, но не гарантируют работу собранных бинарников, парсинг аргументов, обработку сигналов (Ctrl+C) и реальное сетевое взаимодействие в условиях "Black Box".
Цель — реализовать фреймворк на Go, который компилирует текущий код сервера и клиента, запускает их как отдельные процессы и проверяет проксирование трафика.

## 🏗 Decision Log
1.  **Framework: Go `testing` + `os/exec`**
    *   *Why:* Использование bash/python скриптов создаст зависимость от интерпретаторов. Go позволяет писать кросс-платформенные тесты, которые компилируются и запускаются одной командой `go test ./tests/e2e/...`.
    *   *Constraint:* Тесты должны запускаться с флагом `-p 1` или иметь защиту от конфликтов портов.

2.  **Binary Generation: On-the-fly Build**
    *   *Why:* Тестировать нужно именно *текущее* состояние кода, а не старый бинарник в `$PATH`.
    *   *Implementation:* `TestMain` компилирует `cmd/server` и `cmd/client` в `/tmp/revsocks_test_bin/`.

3.  **Network Isolation**
    *   *Why:* Избежать конфликтов с занятыми портами.
    *   *Implementation:* Использование порта `0` (OS выбирает свободный порт) и парсинг реального адреса из логов или `Listener.Addr()`.

## 📦 Modules & Dependencies
*   `tests/e2e/framework.go`: Базовые структуры (TestContext).
*   `tests/e2e/builder.go`: Компиляция единого бинарника RevSocks.
*   `tests/e2e/process.go`: Управление процессами (Start/Stop/WaitForLog).
*   `tests/e2e/target.go`: Echo-сервер для проверки проксирования.
*   `tests/e2e/traffic.go`: SOCKS5 клиент (через `golang.org/x/net/proxy`).
*   `tests/e2e/scenarios_test.go`: Тест-кейсы (Basic, Reconnect).

## 🧪 Testing Strategy
*   **Backend:** Go `testing` package.
*   **Methodology:** Black Box Testing (тест не знает о внутренностях, только CLI аргументы и сетевые сокеты).
*   **Rules:** `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc` (Level 3 Integration).

## 🗺 ROADMAP
| Status | Step | Description |
|:---:|:---|:---|
| 🔴 | [01_Structure](01_Structure_Setup.md) | Подготовка директорий и `TestMain` |
| 🔴 | [02_Builder](02_Binary_Builder.md) | Компиляция бинарников перед тестами |
| 🔴 | [03_Process](03_Process_Manager.md) | Управление процессами (Start/Stop/Logs) |
| 🔴 | [04_Scenarios](04_Scenarios.md) | Реализация тест-кейсов (Connect, Reconnect, Traffic) |

## ✅ Global Checklist
```yaml
todos:
  - id: setup-dir
    content: Создать структуру tests/e2e
    status: pending
    time_estimate: 10m
    dependencies: []
  - id: impl-builder
    content: Реализовать builder.go (ОДИН бинарник, абсолютный путь)
    status: pending
    time_estimate: 20m
    dependencies: [setup-dir]
  - id: impl-proc-mgr
    content: Реализовать process.go для управления процессами
    status: pending
    time_estimate: 30m
    dependencies: [setup-dir]
  - id: impl-traffic
    content: Реализовать traffic.go (SOCKS5 client) + target.go (Echo server)
    status: pending
    time_estimate: 30m
    dependencies: []
  - id: impl-tests
    content: Написать сценарии в scenarios_test.go
    status: pending
    time_estimate: 40m
    dependencies: [impl-builder, impl-proc-mgr, impl-traffic]
```
