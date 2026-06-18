package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mintdesu/portmon/internal/collector"

	"github.com/spf13/cobra"
)

var cleanupOnExit bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the periodic port traffic collector",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		reloadSignals := make(chan os.Signal, 1)
		signal.Notify(reloadSignals, syscall.SIGHUP)
		defer signal.Stop(reloadSignals)

		reload := make(chan struct{}, 1)
		go func() {
			for range reloadSignals {
				select {
				case reload <- struct{}{}:
				default:
				}
			}
		}()

		c, err := collector.New(cfgFile)
		if err != nil {
			return err
		}
		return c.Run(ctx, reload, cleanupOnExit)
	},
}

func init() {
	daemonCmd.Flags().BoolVar(&cleanupOnExit, "cleanup-on-exit", false, "remove portmon iptables rules when daemon exits")
	rootCmd.AddCommand(daemonCmd)
}
