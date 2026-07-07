# 002: yamux stability (ConnectionWriteTimeout 10s→300s)

## Что сделать
Закрыть yamux drop под нагрузкой (та же проблема что ligolo-ng задача 001): `ConnectionWriteTimeout=10s` → write stall под нагрузкой → EOF. Поднять default до 300s синхронно на agent + server (handshake v3 ВАЛИДИРУЕТ совпадение — `ValidateClientSettings`).

## Границы
- Меняем ТОЛЬКО default (флаги `-yamux-timeout` остаются для override).
- Handshake v3 negotiation не трогать — оба дефолта 300s → match без явных флагов.

## Статус
DONE

## Состояние
**сделано:**
- `internal/transport/yamux.go:28` — `DefaultYamuxSettings.WriteTimeout` 10s → 300s (с комментом).
- `cmd/agent/main.go:166` — `defaultYamuxTimeout` 10 → 300.
- `cmd/server/main.go:90` — flag `-yamux-timeout` default 10 → 300.
- Все три default синхронизированы → handshake v3 match без флагов.

**осталось:** стресс-тест (как ligolo chaos ladder) — после деплоя на recon (задача 003).
