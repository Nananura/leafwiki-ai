package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"
	leafmcp "github.com/perber/wiki/internal/mcp"
	"github.com/perber/wiki/internal/wiki"
)

func runMCPServer(dataDir string) {
	// Discard normal logging since MCP uses stdout for communication
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create data directory: %v\n", err)
			os.Exit(1)
		}
	}

	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          dataDir,
		AdminPassword:       "system_mcp_dummy",
		JWTSecret:           "system_mcp_dummy",
		AccessTokenTimeout:  1,
		RefreshTokenTimeout: 1,
		AuthDisabled:        true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Wiki: %v\n", err)
		os.Exit(1)
	}
	defer w.Close()

	s := leafmcp.SetupMCPServer(w)

	// Serve over Stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP Server error: %v\n", err)
		os.Exit(1)
	}
}
