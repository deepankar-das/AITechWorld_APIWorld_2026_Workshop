package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/authsecrets"
)

func main() {
	adminToken := flag.String("admin-token", "", "admin token to seed")
	reviewerToken := flag.String("reviewer-token", "", "reviewer token to seed")
	operatorToken := flag.String("operator-token", "", "operator token to seed")
	flag.Parse()

	tokens := authsecrets.RoleTokens{
		AdminToken:    strings.TrimSpace(*adminToken),
		ReviewerToken: strings.TrimSpace(*reviewerToken),
		OperatorToken: strings.TrimSpace(*operatorToken),
	}
	if tokens.AdminToken == "" && tokens.ReviewerToken == "" && tokens.OperatorToken == "" {
		fmt.Fprintln(os.Stderr, "no tokens provided; pass at least one of --admin-token, --reviewer-token, --operator-token")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := authsecrets.SaveTokens(ctx, tokens); err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed encrypted auth tokens: %v\n", err)
		os.Exit(1)
	}
}
