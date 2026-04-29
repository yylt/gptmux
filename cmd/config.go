package main

import (
	"fmt"
	"os"

	"github.com/yylt/gptmux/pkg/mux/merlin"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Merlin merlin.Config `yaml:"merlin,omitempty"`
	Addr   string        `yaml:"address"`
	Debug  bool          `yaml:"debug"`
}

// LoadConfigmap reads configmap data from config-path
func LoadConfigmap(fp string) (*Config, error) {
	var (
		cfg = &Config{}
	)
	configmapBytes, err := os.ReadFile(fp)
	if nil != err {
		return nil, fmt.Errorf("failed to read config file %s, error: %v", fp, err)
	}

	err = yaml.Unmarshal(configmapBytes, &cfg)
	if nil != err {
		return nil, fmt.Errorf("failed to parse configmap, error: %v", err)
	}

	return cfg, nil
}

func (c *Config) SaveConfig(fp string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config, error: %v", err)
	}
	if err := os.WriteFile(fp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s, error: %v", fp, err)
	}
	return nil
}
