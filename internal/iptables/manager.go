package iptables

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mintdesu/portmon/internal/config"
)

const (
	ChainIn  = "PORTMON_IN"
	ChainOut = "PORTMON_OUT"
)

type Snapshot struct {
	In  map[string]uint64
	Out map[string]uint64
}

type Manager struct {
	path string
}

func NewManager(path string) *Manager {
	if path == "" {
		path = config.DefaultIptables
	}
	return &Manager{path: path}
}

func (m *Manager) Setup(ctx context.Context, cfg *config.Config) error {
	if err := m.ensureChain(ctx, ChainIn); err != nil {
		return err
	}
	if err := m.ensureChain(ctx, ChainOut); err != nil {
		return err
	}
	if err := m.ensureJump(ctx, "INPUT", "-i", cfg.Interface, ChainIn); err != nil {
		return err
	}
	if err := m.ensureJump(ctx, "OUTPUT", "-o", cfg.Interface, ChainOut); err != nil {
		return err
	}

	for _, port := range cfg.Ports {
		for _, proto := range []string{"tcp", "udp"} {
			if err := m.ensurePortRule(ctx, ChainIn, proto, "--dports", port.Port); err != nil {
				return err
			}
			if err := m.ensurePortRule(ctx, ChainOut, proto, "--sports", port.Port); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) Cleanup(ctx context.Context, cfg *config.Config) error {
	var errs []string
	if err := m.deleteJump(ctx, "INPUT", "-i", cfg.Interface, ChainIn); err != nil {
		errs = append(errs, err.Error())
	}
	if err := m.deleteJump(ctx, "OUTPUT", "-o", cfg.Interface, ChainOut); err != nil {
		errs = append(errs, err.Error())
	}
	for _, chain := range []string{ChainIn, ChainOut} {
		if _, err := m.run(ctx, "-F", chain); err != nil && !isMissingChain(err.Error()) {
			errs = append(errs, err.Error())
		}
		if _, err := m.run(ctx, "-X", chain); err != nil && !isMissingChain(err.Error()) {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup iptables: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (m *Manager) Reconcile(ctx context.Context, cfg *config.Config) error {
	if err := m.Cleanup(ctx, cfg); err != nil {
		return err
	}
	return m.Setup(ctx, cfg)
}

func (m *Manager) Snapshot(ctx context.Context) (Snapshot, error) {
	inCounters, err := m.ReadChain(ctx, ChainIn)
	if err != nil {
		return Snapshot{}, err
	}
	outCounters, err := m.ReadChain(ctx, ChainOut)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{In: inCounters, Out: outCounters}, nil
}

func (m *Manager) ReadChain(ctx context.Context, chain string) (map[string]uint64, error) {
	output, err := m.run(ctx, "-L", chain, "-v", "-n", "-x")
	if err != nil {
		return nil, err
	}
	counters, err := ParseCounters(output)
	if err != nil {
		return nil, fmt.Errorf("parse %s counters: %w", chain, err)
	}
	return counters, nil
}

func (m *Manager) ensureChain(ctx context.Context, chain string) error {
	_, err := m.run(ctx, "-N", chain)
	if err == nil || isAlreadyExists(err.Error()) {
		return nil
	}
	return err
}

func (m *Manager) ensureJump(ctx context.Context, baseChain string, directionFlag string, iface string, targetChain string) error {
	args := []string{baseChain, directionFlag, iface, "-j", targetChain}
	if m.exists(ctx, args...) {
		return nil
	}
	insert := withPrefix("-I", args...)
	_, err := m.run(ctx, insert...)
	return err
}

func (m *Manager) deleteJump(ctx context.Context, baseChain string, directionFlag string, iface string, targetChain string) error {
	args := []string{baseChain, directionFlag, iface, "-j", targetChain}
	for m.exists(ctx, args...) {
		del := withPrefix("-D", args...)
		if _, err := m.run(ctx, del...); err != nil {
			if isMissingRule(err.Error()) {
				return nil
			}
			return err
		}
	}
	return nil
}

func (m *Manager) ensurePortRule(ctx context.Context, chain string, proto string, portFlag string, port config.PortRange) error {
	comment := "portmon:" + port.String()
	args := []string{
		chain,
		"-p", proto,
		"-m", "multiport",
		portFlag, port.IptablesSpec(),
		"-m", "comment",
		"--comment", comment,
		"-j", "RETURN",
	}
	if m.exists(ctx, args...) {
		return nil
	}
	appendArgs := withPrefix("-A", args...)
	_, err := m.run(ctx, appendArgs...)
	return err
}

func (m *Manager) exists(ctx context.Context, args ...string) bool {
	checkArgs := withPrefix("-C", args...)
	_, err := m.run(ctx, checkArgs...)
	return err == nil
}

func (m *Manager) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, m.path, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return text, fmt.Errorf("%s %s: %w", m.path, strings.Join(args, " "), err)
		}
		return text, fmt.Errorf("%s %s: %w: %s", m.path, strings.Join(args, " "), err, text)
	}
	return text, nil
}

func isAlreadyExists(message string) bool {
	return strings.Contains(message, "Chain already exists") || strings.Contains(message, "File exists")
}

func isMissingChain(message string) bool {
	return strings.Contains(message, "No chain/target/match by that name") ||
		strings.Contains(message, "No chain") ||
		strings.Contains(message, "does not exist")
}

func isMissingRule(message string) bool {
	return isMissingChain(message) ||
		strings.Contains(message, "Bad rule") ||
		strings.Contains(message, "No such file or directory")
}

func withPrefix(prefix string, args ...string) []string {
	out := make([]string, 0, 1+len(args))
	out = append(out, prefix)
	out = append(out, args...)
	return out
}
