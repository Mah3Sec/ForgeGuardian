package main

import (
	"fmt"
	"os"

	"github.com/mah3sec/forgeguardian/internal/api/server"
)

func main() {
	cfg := server.LoadConfig()
	if err := server.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
