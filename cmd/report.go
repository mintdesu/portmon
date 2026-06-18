package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/mintdesu/portmon/internal/storage"

	"github.com/spf13/cobra"
)

var (
	reportFrom    string
	reportTo      string
	reportByOwner bool
	reportMonthly bool
)

var reportCmd = &cobra.Command{
	Use:   "report [today]",
	Short: "Summarize recorded traffic from CSV samples",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if reportByOwner && reportMonthly {
			return fmt.Errorf("--by-owner and --monthly cannot be used together")
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		from, to, err := reportRange(args)
		if err != nil {
			return err
		}

		rows, err := storage.Summarize(cfg.DataDir, storage.ReportOptions{
			From:    from,
			To:      to,
			ByOwner: reportByOwner,
			Monthly: reportMonthly,
		})
		if err != nil {
			return err
		}
		printRows(os.Stdout, rows)
		return nil
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportFrom, "from", "", "start date in YYYY-MM-DD")
	reportCmd.Flags().StringVar(&reportTo, "to", "", "end date in YYYY-MM-DD")
	reportCmd.Flags().BoolVar(&reportByOwner, "by-owner", false, "group report rows by owner")
	reportCmd.Flags().BoolVar(&reportMonthly, "monthly", false, "group report rows by month")
	rootCmd.AddCommand(reportCmd)
}

func reportRange(args []string) (time.Time, time.Time, error) {
	if len(args) == 1 && args[0] == "today" {
		now := time.Now()
		return now, now, nil
	}
	if len(args) == 1 {
		return time.Time{}, time.Time{}, fmt.Errorf("unknown report preset %q", args[0])
	}

	var from time.Time
	var to time.Time
	var err error
	if reportFrom != "" {
		from, err = parseDate(reportFrom)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if reportTo != "" {
		to, err = parseDate(reportTo)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if from.IsZero() && to.IsZero() {
		now := time.Now()
		return now, now, nil
	}
	if from.IsZero() {
		from = to
	}
	if to.IsZero() {
		to = from
	}
	return from, to, nil
}

func parseDate(input string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", input, time.Local)
}
