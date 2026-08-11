//go:build windows

package main

// Windows のコンソールは既定のコードページが UTF-8 ではないため、
// 日本語やスート記号がそのままでは化ける。ここで UTF-8 に切り替え、
// ついでに ANSI エスケープ (色) を有効にする。

import (
	"syscall"
	"unsafe"
)

const (
	codePageUTF8                 = 65001
	enableVirtualTerminalProcess = 0x0004
)

// setupConsole は色表示が使えるかどうかを返す。
func setupConsole() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleCP := kernel32.NewProc("SetConsoleCP")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	_, _, _ = setConsoleOutputCP.Call(uintptr(codePageUTF8))
	_, _, _ = setConsoleCP.Call(uintptr(codePageUTF8))

	handle, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}

	var mode uint32
	if ret, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode))); ret == 0 {
		// リダイレクト中などコンソールでない場合は色を使わない。
		return false
	}
	ret, _, _ := setConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcess))
	return ret != 0
}
