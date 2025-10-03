package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PathConfig defines a JSONPath into DNS record mapping
type PathConfig struct {
	Path string `json:"path" yaml:"path"`
	Type string `json:"type" yaml:"type"`
}

// WatchConfig describes a watch for the DNSEndpoint mapping
type WatchConfig struct {
	Group          string       `json:"group" yaml:"group"`
	Version        string       `json:"version" yaml:"version"`
	Kind           string       `json:"kind" yaml:"kind"`
	Namespace      string       `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	RecordTemplate string       `json:"recordTemplate" yaml:"recordTemplate"`
	Paths          []PathConfig `json:"paths" yaml:"paths"`
}

// Config defines the root configuration definitioon
type Config struct {
	Watches []WatchConfig `json:"watches" yaml:"watches"`
}

// Load handles the loading of the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	fmt.Printf("%+v\n", cfg)

	return &cfg, nil
}
