package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "m",
	Short: "M CLI - manage M services",
	Long:  `M is a CLI tool for managing M services including the server and configuration.`,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(configCmd)
}
