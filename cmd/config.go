package main

import (
	"fmt"
	"os"

	"github.com/yylt/chatmux/pkg/merlin"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Merlin merlin.Config `yaml:"merlin"`
	Addr   string        `yaml:"address"`
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
