package cmd

import (
	"context"
	"time"

	"portmon/internal/iptables"

	"github.com/spf13/cobra"
)

var iptablesCmd = &cobra.Command{
	Use:   "iptables",
	Short: "Manage portmon iptables chains and rules",
}

var iptablesSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create or repair portmon iptables rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return iptables.NewManager(cfg.IptablesPath).Setup(ctx, cfg)
	},
}

var iptablesCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove portmon iptables rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return iptables.NewManager(cfg.IptablesPath).Cleanup(ctx, cfg)
	},
}

func init() {
	iptablesCmd.AddCommand(iptablesSetupCmd, iptablesCleanupCmd)
	rootCmd.AddCommand(iptablesCmd)
}
