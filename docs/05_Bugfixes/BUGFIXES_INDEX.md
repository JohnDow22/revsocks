# Bugfixes Index - RevSocks

Централизованный индекс всех задокументированных багфиксов.

---

## 📅 2026-01-09

### Port Leak & Session Race Condition

📄 [2026_01_09_PORT_LEAK_RACE_CONDITION.md](2026_01_09_PORT_LEAK_RACE_CONDITION.md)  
**Приоритет:** 🔴 CRITICAL  

Исправлена утечка портов и гонка между старой и новой сессией агента (SessionManager + generation tokens).

---

### Critical Bugfix Release 2.3 (9 багов)

📄 [2026_01_09_CRITICAL_BUGFIX_2_3.md](2026_01_09_CRITICAL_BUGFIX_2_3.md)  
**Приоритет:** 🔴 CRITICAL  

Комплексный релиз, закрывающий 9 багов:
- Crash Prevention (proxyauth, `net.Dial`, длинный пароль);
- Resource Leaks (legacy `sessions[]`, HTTP Body);
- Logic Bugs (failover round-robin, `nil` вместо `error`);
- Security (секреты в логах).

---

## 📋 Быстрый поиск

| Багфикс | Дата | Приоритет | Документ |
|--------|------|-----------|----------|
| Port Leak & Session Race Condition | 2026-01-09 | 🔴 CRITICAL | [2026_01_09_PORT_LEAK_RACE_CONDITION.md](2026_01_09_PORT_LEAK_RACE_CONDITION.md) |
| Critical Bugfix Release 2.3 | 2026-01-09 | 🔴 CRITICAL | [2026_01_09_CRITICAL_BUGFIX_2_3.md](2026_01_09_CRITICAL_BUGFIX_2_3.md) |

