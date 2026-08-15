//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func diskFreeGB() (float64, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetDiskFreeSpaceExW")
	dir, err := syscall.UTF16PtrFromString(os.TempDir())
	if err != nil {
		return 0, err
	}
	var freeBytesAvailable uint64
	r, _, e := proc.Call(
		uintptr(unsafe.Pointer(dir)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		0,
		0,
	)
	if r == 0 {
		return 0, e
	}
	return float64(freeBytesAvailable) / (1024 * 1024 * 1024), nil
}
