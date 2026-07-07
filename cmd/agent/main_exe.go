//go:build !cgo

// main_exe.go — EXE entry point (build !cgo).
// DLL build (cgo+windows) использует havoc_dll.go (DllMain → mainImpl).

package main

func main() {
	mainImpl()
}
