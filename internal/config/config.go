package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath       = "/etc/portmon/config.yaml"
	DefaultDataDir          = "/var/lib/portmon"
	DefaultIntervalSec      = 60
	DefaultIptables         = "iptables"
	DefaultLogRetentionDays = 30
)

type Config struct {
	Interface        string       `yaml:"interface"`
	Ports            []PortConfig `yaml:"ports"`
	Interval         int          `yaml:"interval"`
	DataDir          string       `yaml:"data_dir"`
	LogRetentionDays int          `yaml:"log_retention_days"`
	CleanupOnExit    bool         `yaml:"cleanup_on_exit"`
	IptablesPath     string       `yaml:"iptables_path"`
}

type PortConfig struct {
	Port  PortRange `yaml:"port"`
	Name  string    `yaml:"name"`
	Owner string    `yaml:"owner"`
}

type PortRange struct {
	Start int
	End   int
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := Config{LogRetentionDays: DefaultLogRetentionDays}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.Interval == 0 {
		c.Interval = DefaultIntervalSec
	}
	if c.DataDir == "" {
		c.DataDir = DefaultDataDir
	}
	if c.IptablesPath == "" {
		c.IptablesPath = DefaultIptables
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Interface) == "" {
		return errors.New("config interface is required")
	}
	if c.Interval <= 0 {
		return fmt.Errorf("config interval must be positive, got %d", c.Interval)
	}
	if c.LogRetentionDays < 0 {
		return fmt.Errorf("config log_retention_days must be zero or positive, got %d", c.LogRetentionDays)
	}
	if len(c.Ports) == 0 {
		return errors.New("config ports must contain at least one port")
	}

	seen := make(map[string]struct{}, len(c.Ports))
	for i, port := range c.Ports {
		if err := port.Port.Validate(); err != nil {
			return fmt.Errorf("config ports[%d]: %w", i, err)
		}
		key := port.Port.String()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("config ports[%d]: duplicate port %s", i, key)
		}
		for j := 0; j < i; j++ {
			if port.Port.Overlaps(c.Ports[j].Port) {
				return fmt.Errorf("config ports[%d] %s overlaps ports[%d] %s", i, key, j, c.Ports[j].Port.String())
			}
		}
		seen[key] = struct{}{}
	}

	return nil
}

func (p *PortRange) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("port must be a scalar")
	}

	parsed, err := ParsePortRange(value.Value)
	if err != nil {
		return err
	}
	*p = parsed
	return nil
}

func ParsePortRange(input string) (PortRange, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return PortRange{}, errors.New("port cannot be empty")
	}

	separator := "-"
	if strings.Contains(value, ":") {
		separator = ":"
	}

	parts := strings.Split(value, separator)
	if len(parts) > 2 {
		return PortRange{}, fmt.Errorf("invalid port range %q", input)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid port %q", input)
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return PortRange{}, fmt.Errorf("invalid port range %q", input)
		}
	}

	r := PortRange{Start: start, End: end}
	if err := r.Validate(); err != nil {
		return PortRange{}, err
	}
	return r, nil
}

func (p PortRange) Validate() error {
	if p.Start < 1 || p.Start > 65535 {
		return fmt.Errorf("port start must be between 1 and 65535, got %d", p.Start)
	}
	if p.End < 1 || p.End > 65535 {
		return fmt.Errorf("port end must be between 1 and 65535, got %d", p.End)
	}
	if p.End < p.Start {
		return fmt.Errorf("port range end %d is smaller than start %d", p.End, p.Start)
	}
	return nil
}

func (p PortRange) String() string {
	if p.Start == p.End {
		return strconv.Itoa(p.Start)
	}
	return fmt.Sprintf("%d-%d", p.Start, p.End)
}

func (p PortRange) IptablesSpec() string {
	if p.Start == p.End {
		return strconv.Itoa(p.Start)
	}
	return fmt.Sprintf("%d:%d", p.Start, p.End)
}

func (p PortRange) Overlaps(other PortRange) bool {
	return p.Start <= other.End && other.Start <= p.End
}
