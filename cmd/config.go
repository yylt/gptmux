package main

import (
	"fmt"
	"os"

	"github.com/yylt/gptmux/mux/claude"
	"github.com/yylt/gptmux/mux/deepseek"
	"github.com/yylt/gptmux/mux/merlin"
	"github.com/yylt/gptmux/mux/ollama"
	"github.com/yylt/gptmux/mux/rkllm"
	"github.com/yylt/gptmux/pkg/box"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Merlin   merlin.Config  `yaml:"merlin,omitempty"`
	Claude   claude.Conf    `yaml:"claude,omitempty"`
	Ollama   ollama.Config  `yaml:"ollama,omitempty"`
	Deepseek deepseek.Conf  `yaml:"deepseek,omitempty"`
	Notify   box.NotifyConf `yaml:"notify,omitempty"`
	Rkllm    rkllm.Conf     `yaml:"rkllm,omitempty"`
	Addr     string         `yaml:"address"`
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
