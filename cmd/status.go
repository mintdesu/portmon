package cmd

import (
	"context"
	"os"
	"time"

	"github.com/mintdesu/portmon/internal/iptables"
	"github.com/mintdesu/portmon/internal/storage"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current cumulative iptables counters for configured ports",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		snapshot, err := iptables.NewManager(cfg.IptablesPath).Snapshot(ctx)
		if err != nil {
			return err
		}

		rows := make([]storage.SummaryRow, 0, len(cfg.Ports))
		for _, port := range cfg.Ports {
			label := port.Port.String()
			rows = append(rows, storage.SummaryRow{
				Key:      label,
				Name:     port.Name,
				Owner:    port.Owner,
				BytesIn:  snapshot.In[label],
				BytesOut: snapshot.Out[label],
			})
		}
		cmd.Println("Counters are cumulative since the current iptables rules were created.")
		printRows(os.Stdout, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
