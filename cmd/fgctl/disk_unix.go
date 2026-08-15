//go:build !windows

package main

import (
	"os"
	"syscall"
)

func diskFreeGB() (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(os.TempDir(), &stat); err != nil {
		return 0, err
	}
	return float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024), nil
}
