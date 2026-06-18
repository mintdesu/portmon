package cmd

import (
	"portmon/internal/config"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "portmon",
	Short: "Monitor network traffic by configured port using iptables counters",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", config.DefaultConfigPath, "path to config.yaml")
}

func loadConfig() (*config.Config, error) {
	return config.Load(cfgFile)
}
