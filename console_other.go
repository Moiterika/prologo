//go:build !windows

package main

// Windows 以外の端末は UTF-8 と ANSI エスケープをそのまま扱えるので何もしない。
func setupConsole() bool { return true }
