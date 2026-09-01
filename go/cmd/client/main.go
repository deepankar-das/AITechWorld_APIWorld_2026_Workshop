/**
 * Author: Deepankar Das
 */

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthropics/enforcer/internal/client"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   ENFORCER — Client Agent         ║")
	fmt.Println("╚══════════════════════════════════════╝")

	agent := client.NewClientAgent()
	if err := agent.Start(); err != nil {
		slog.Error("Client agent start failed", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutting down client agent")
	agent.Stop()
}
