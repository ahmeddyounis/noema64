package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ahmedyounis/noema64/internal/storage"
	"github.com/ahmedyounis/noema64/internal/uci"
)

func main() {
	settings, err := storage.LoadSettings("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load settings:", err)
		settings = storage.DefaultSettings()
	}
	server := uci.NewServer(os.Stdin, os.Stdout, os.Stderr, settings)
	if err := server.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
