# 001: agent DLL args adapter (Havoc DllSpawn, как ligolo-ng)

## Что сделать
Адаптировать `cmd/agent` под c-shared DLL для Havoc DllSpawn с динамической передачей args через lpvReserved (паттерн ligolo-ng `havoc_dll.go`). Заменить старую kost v2.8 DLL в HavoX soks/ (где DllMain игнорил lpvReserved, target хардкод).

## Границы
- НЕ трогать baked.go / stealth build (DLL build = isStealth=false, args из lpvReserved).
- НЕ делать trampoline (-Wl,-e ломал reflective-load у ligolo — тот же Go runtime).
- soks.py НЕ трогать (уже кодирует base64 args).

## Статус
DONE

## Состояние
**сделано:**
- `cmd/agent/main.go:148` — `func main()` → `func mainImpl()` (точка входа: `agent.StartBeaconLoop(cfg)` / `ConnectWebsocket(cfg)`).
- `cmd/agent/havoc_dll.go` (новый, `//go:build cgo && windows`) — `//export DllMain`: lpvReserved → C.GoString → base64 decode → `os.Args = ["revsocks", ...]` → `mainImpl()`. atomic CAS guard. Нет `-connect` → return.
- `cmd/agent/main_exe.go` (новый, `//go:build !cgo`) — `func main(){ mainImpl() }`.
- `Makefile` — target `agent-dll`: `CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -ldflags "-s -w" -o dist/revsocks-agent.dll ./cmd/agent`.
- Build: agent EXE 12M (linux), DLL 8.0M (windows), exports `DllMain` + `_cgo_dummy_export`.
- DLL скопирована в `HavoX/client/commander/modules/soks/bin/revsocks-agent.dll` (замена kost 5.2M).

**осталось:** —
