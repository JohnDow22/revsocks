# RevSocks Bugfix & Stabilization Plan

```yaml
todos:
  # Группа Б1: Логические ошибки
  - id: B1-1
    content: "Fix busy-loop в rdns.go при разрыве сессии"
    status: pending
    time_estimate: "30 мин"
    dependencies: []
  - id: B1-2
    content: "Fix игнорирование ошибок strconv.Atoi"
    status: pending
    time_estimate: "15 мин"
    dependencies: []
  - id: B1-3
    content: "Fix IPv6 parsing (net.SplitHostPort вместо Split)"
    status: pending
    time_estimate: "20 мин"
    dependencies: []
  - id: B1-4
    content: "Fix race condition в h.sessions слайсе"
    status: pending
    time_estimate: "30 мин"
    dependencies: []
  
  # Группа А1: Сетевой протокол
  - id: A1-1
    content: "Добавить length-prefixed AgentID в протокол"
    status: pending
    time_estimate: "1 час"
    dependencies: [B1-4]
  - id: A1-2
    content: "Заменить time.Sleep на ACK handshake"
    status: pending
    time_estimate: "1.5 часа"
    dependencies: [A1-1]
  - id: A1-3
    content: "Добавить подтверждение авторизации (OK/FAIL)"
    status: pending
    time_estimate: "1 час"
    dependencies: [A1-2]
  
  # Группа В1: Архитектура
  - id: V1-1
    content: "Добавить graceful shutdown с context"
    status: pending
    time_estimate: "2 часа"
    dependencies: [A1-3]
  - id: V1-2
    content: "Вынести дублирующий код в хелперы"
    status: pending
    time_estimate: "1.5 часа"
    dependencies: [V1-1]
  - id: V1-3
    content: "Разделить config runtime/build-time"
    status: pending
    time_estimate: "1 час"
    dependencies: [V1-2]
  
  # Тестирование
  - id: T-1
    content: "Написать unit-тесты для критичных функций"
    status: pending
    time_estimate: "2 часа"
    dependencies: [V1-3]
  - id: T-2
    content: "Интеграционный тест reconnect сценария"
    status: pending
    time_estimate: "1 час"
    dependencies: [T-1]
```

---

## Цель

Исправить CONFIRMED баги и THEORETICAL риски в RevSocks без увеличения технического долга.
Результат: стабильная работа при разрывах соединения, корректный протокол handshake, graceful shutdown.

---

## Decision Log (Почему так)

### 1. Length-prefixed AgentID вместо fixed-size
- **Причина**: Один `Read()` может вернуть "обрубок" при TCP-фрагментации.
- **Решение**: `[1 byte length][agentID bytes]` — надёжное чтение.
- **Альтернатива отвергнута**: Fixed 64 bytes — waste bandwidth + всё равно race при заполнении.

### 2. ACK handshake вместо Sleep(1s)
- **Причина**: Sleep не гарантирует синхронизацию, замедляет reconnect.
- **Решение**: Сервер шлёт `OK\n` после валидации пароля → клиент стартует yamux.
- **Backward compatibility**: Клиент v2 + Сервер v1 = timeout → reconnect.

### 3. Generation token для race protection
- **Причина**: Cleanup старой сессии может закрыть новую с тем же agentID.
- **Решение**: Уже реализовано в SessionManager (generation counter) — проверить корректность.

### 4. Context-based graceful shutdown
- **Причина**: Ctrl+C рвёт соединения жёстко, ресурсы не очищаются.
- **Решение**: `signal.NotifyContext` + propagation через context.

---

## Матрица Зависимостей

| Модуль | Затронут | Изменения |
|--------|----------|-----------|
| `rserver.go` | ✅ | Protocol, SessionManager, shutdown |
| `rclient.go` | ✅ | Protocol, ACK handshake, shutdown |
| `rdns.go` | ✅ | Busy-loop fix |
| `main.go` | ✅ | Graceful shutdown, error handling |
| `yamux_config.go` | ⚪ | Без изменений |
| `build_stealth.sh` | ⚪ | Без изменений |

---

## Стратегия Тестирования

**Уровень**: Level 1-2 (Unit + Integration для критичных путей)

Согласно `.cursor/rules/Dev_2.0/quality/Testing/Gemini3_Test_rules/Testing_Decision_Matrix.mdc`:
- Размер: ~1500 LOC → Level 2 (Integration)
- Критичность: Высокая (сетевой протокол) → Unit на парсеры

**Тесты**:
1. `TestParseProxyAuth` — уже в main.go, расширить edge cases
2. `TestExtractAgentIP` — IPv4, IPv6, edge cases
3. `TestProtocolHandshake` — mock TCP, length-prefixed read
4. `TestSessionManagerRace` — concurrent register/unregister
5. `TestGracefulShutdown` — signal handling

---

## ROADMAP

| Этап | Файл | Описание | Статус | Зависимости |
|------|------|----------|--------|-------------|
| 01 | [01_Fix_Panics.md](01_Fix_Panics.md) | Crash Prevention (паники) | 🟢 DONE | - |
| 02 | [02_Fix_Leaks.md](02_Fix_Leaks.md) | Resource Leaks (утечки) | 🟢 DONE | - |
| 03 | [03_Fix_Logic.md](03_Fix_Logic.md) | Группа Б1: Логические ошибки | 🟢 DONE | - |
| 04 | [04_Fix_Security.md](04_Fix_Security.md) | Группа А1: Протокол и синхронизация | 🟢 DONE | 03 |
| 05 | [05_Testing.md](05_Testing.md) | Unit + Integration тесты | 🟢 DONE | 01-04 |

**Примечание**: Группа В1 (Graceful Shutdown) отложена — требует значительного рефакторинга. 
Приоритет: стабилизация протокола (Б1 + А1) → тестирование → В1 в отдельной итерации.

---

## База знаний (из Zep)

Релевантный опыт найден в базе:
1. **Race condition cleanup** — generation token решает проблему
2. **Handshake protocol** — уже расширен `<password>\n<agentID>`
3. **SessionManager** — уже трекает сессии с port caching
4. **yamuxConfig** — вынесен в общий файл

**Вывод**: Часть работы уже сделана (см. rserver.go:26-158). Нужно:
- Проверить корректность реализации
- Добавить недостающие фиксы (DNS busy-loop, strconv errors, IPv6)
- Добавить ACK вместо Sleep

---

## Anti-patterns (чего НЕ делать)

1. **НЕ** использовать глобальные переменные для новой логики
2. **НЕ** добавлять Sleep для "надёжности"
3. **НЕ** игнорировать ошибки Read/Write в сетевом коде
4. **НЕ** удалять существующие логи отладки
5. **НЕ** менять публичный API без backward compatibility
6. **НЕ** писать тесты на конкретную реализацию (тестируем поведение)

---

## Quick Start для LLM

```text
Твоя роль — Go Developer, специализирующийся на сетевом программировании.
Твоя задача — последовательно выполнить этапы 03 и 05 этого плана.

Контекст:
- Проект RevSocks — reverse SOCKS5 proxy на Go
- Используется yamux для мультиплексирования
- SessionManager уже реализован (rserver.go:26-158)
- Handshake: password + \n + agentID

Читай этапы в порядке: 03_Fix_Logic → 05_Testing
```
