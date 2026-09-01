/**
 * Author: Deepankar Das
 */

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/anthropics/enforcer/internal/central"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   ENFORCER — Management Hub       ║")
	fmt.Println("╚══════════════════════════════════════╝")

	certDir := envOr("CERT_DIR", "/etc/enforcer/certs")
	policyDir := envOr("AA_POLICY_DIR", "/etc/enforcer")
	clientPort := 9200
	adminPort := 9201

	server, err := central.NewCentralServer(certDir, clientPort, adminPort)
	if err != nil {
		slog.Error("Management Hub failed to start — PostgreSQL required for audit aggregation", "error", err)
		os.Exit(1)
	}

	policyPath := policyDir + "/default.yaml"
	if err := server.LoadPolicy(policyPath); err != nil {
		slog.Error("Management Hub failed to load policy", "path", policyPath, "error", err)
		os.Exit(1)
	}

	slog.Info("Starting Management Hub", "sentinel_port", clientPort, "console_port", adminPort)
	if err := server.Start(); err != nil {
		slog.Error("Management Hub failed", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
