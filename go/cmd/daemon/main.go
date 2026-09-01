/**
 * Author: Deepankar Das
 */

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/anthropics/enforcer/internal/daemon"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║      ENFORCER — Daemon            ║")
	fmt.Printf("║      Version: %-23s║\n", Version)
	fmt.Println("╚══════════════════════════════════════╝")

	if err := daemon.StartDaemon(); err != nil {
		slog.Error("Daemon failed", "error", err)
		os.Exit(1)
	}
}
