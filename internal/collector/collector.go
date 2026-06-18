package collector

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mintdesu/portmon/internal/config"
	"github.com/mintdesu/portmon/internal/iptables"
	"github.com/mintdesu/portmon/internal/storage"
)

type Collector struct {
	configPath string
	cfg        *config.Config
	manager    *iptables.Manager
	state      storage.State
}

func New(configPath string) (*Collector, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	state, err := storage.LoadState(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	return &Collector{
		configPath: configPath,
		cfg:        cfg,
		manager:    iptables.NewManager(cfg.IptablesPath),
		state:      state,
	}, nil
}

func (c *Collector) Run(ctx context.Context, reload <-chan struct{}, cleanupOnExit bool) error {
	if err := c.manager.Setup(ctx, c.cfg); err != nil {
		return err
	}
	slog.Info("portmon daemon started", "interface", c.cfg.Interface, "interval_seconds", c.cfg.Interval, "data_dir", c.cfg.DataDir)
	if cleanupOnExit || c.cfg.CleanupOnExit {
		defer func() {
			if err := c.manager.Cleanup(context.Background(), c.cfg); err != nil {
				slog.Warn("cleanup on exit failed", "error", err)
			}
		}()
	}

	if len(c.state.Ports) == 0 {
		if err := c.seedState(ctx); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(time.Duration(c.cfg.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("portmon daemon stopping")
			return storage.SaveState(c.cfg.DataDir, c.state)
		case <-reload:
			if err := c.Reload(ctx); err != nil {
				slog.Warn("reload failed; keeping previous configuration", "error", err)
				continue
			}
			ticker.Reset(time.Duration(c.cfg.Interval) * time.Second)
		case <-ticker.C:
			if err := c.CollectOnce(ctx); err != nil {
				slog.Warn("collection failed; will retry on next interval", "error", err)
				continue
			}
		}
	}
}

func (c *Collector) Reload(ctx context.Context) error {
	nextCfg, err := config.Load(c.configPath)
	if err != nil {
		return err
	}
	nextManager := iptables.NewManager(nextCfg.IptablesPath)
	if err := c.manager.Cleanup(ctx, c.cfg); err != nil {
		slog.Warn("cleanup during reload failed; continuing with setup", "error", err)
	}
	if err := nextManager.Setup(ctx, nextCfg); err != nil {
		return err
	}

	if nextCfg.DataDir != c.cfg.DataDir {
		state, err := storage.LoadState(nextCfg.DataDir)
		if err != nil {
			return err
		}
		c.state = state
	}

	c.cfg = nextCfg
	c.manager = nextManager
	if err := c.seedState(ctx); err != nil {
		return err
	}
	slog.Info("configuration reloaded", "interface", c.cfg.Interface, "interval_seconds", c.cfg.Interval, "data_dir", c.cfg.DataDir)
	return nil
}

func (c *Collector) CollectOnce(ctx context.Context) error {
	if err := c.manager.Setup(ctx, c.cfg); err != nil {
		return err
	}

	snapshot, err := c.manager.Snapshot(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	samples, nextState := c.samplesFromSnapshot(now, snapshot)
	if err := storage.AppendSamples(c.cfg.DataDir, samples); err != nil {
		return err
	}
	c.state = nextState
	if err := storage.SaveState(c.cfg.DataDir, c.state); err != nil {
		return err
	}
	if removed, err := storage.CleanupOldCSVs(c.cfg.DataDir, c.cfg.LogRetentionDays, now); err != nil {
		slog.Warn("old csv cleanup failed", "error", err)
	} else if removed > 0 {
		slog.Info("old csv files removed", "count", removed, "retention_days", c.cfg.LogRetentionDays)
	}
	slog.Debug("collection completed", "samples", len(samples), "data_dir", c.cfg.DataDir)
	return nil
}

func (c *Collector) seedState(ctx context.Context) error {
	snapshot, err := c.manager.Snapshot(ctx)
	if err != nil {
		return err
	}
	c.state = stateFromSnapshot(time.Now(), c.cfg, snapshot)
	return storage.SaveState(c.cfg.DataDir, c.state)
}

func (c *Collector) samplesFromSnapshot(timestamp time.Time, snapshot iptables.Snapshot) ([]storage.Sample, storage.State) {
	nextState := stateFromSnapshot(timestamp, c.cfg, snapshot)
	samples := make([]storage.Sample, 0, len(c.cfg.Ports))

	for _, port := range c.cfg.Ports {
		label := port.Port.String()
		previous, hasPrevious := c.state.Ports[label]
		current := nextState.Ports[label]

		var bytesIn uint64
		var bytesOut uint64
		if hasPrevious {
			bytesIn = delta(previous.BytesIn, current.BytesIn)
			bytesOut = delta(previous.BytesOut, current.BytesOut)
		}

		samples = append(samples, storage.Sample{
			Timestamp: timestamp,
			Port:      label,
			Name:      port.Name,
			Owner:     port.Owner,
			BytesIn:   bytesIn,
			BytesOut:  bytesOut,
		})
	}

	return samples, nextState
}

func stateFromSnapshot(timestamp time.Time, cfg *config.Config, snapshot iptables.Snapshot) storage.State {
	state := storage.State{
		UpdatedAt: timestamp,
		Ports:     make(map[string]storage.StateCounter, len(cfg.Ports)),
	}
	for _, port := range cfg.Ports {
		label := port.Port.String()
		state.Ports[label] = storage.StateCounter{
			BytesIn:  snapshot.In[label],
			BytesOut: snapshot.Out[label],
		}
	}
	return state
}

func delta(previous uint64, current uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return current
}

func RunOnce(configPath string) error {
	collector, err := New(configPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := collector.CollectOnce(ctx); err != nil {
		return fmt.Errorf("collect once: %w", err)
	}
	return nil
}
