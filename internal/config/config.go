package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interests []Interest    `yaml:"interests"`
	Scoring   ScoringConfig `yaml:"scoring"`
	APIs      APIConfig     `yaml:"apis"`
	Fetch     FetchConfig   `yaml:"fetch"`
	Daemon    DaemonConfig  `yaml:"daemon"`
	Reddit    RedditConfig  `yaml:"reddit"`
}

type Interest struct {
	Topic    string   `yaml:"topic"`
	Weight   float64  `yaml:"weight"`
	Keywords []string `yaml:"keywords,omitempty"`
}

type ScoringConfig struct {
	Community float64 `yaml:"community"`
	Relevance float64 `yaml:"relevance"`
	Novelty   float64 `yaml:"novelty"`
}

type APIConfig struct {
	LLMProvider string `yaml:"llm_provider"`
	LLMModel    string `yaml:"llm_model"`
	OpenAIKey   string `yaml:"openai_key,omitempty"`
}

type FetchConfig struct {
	Concurrency    int    `yaml:"concurrency"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	UserAgent      string `yaml:"user_agent"`
}

type DaemonConfig struct {
	IntervalHours int `yaml:"interval_hours"`
}

type RedditConfig struct {
	Subreddits []string `yaml:"subreddits"`
}

func Default() *Config {
	return &Config{
		Interests: []Interest{},
		Scoring: ScoringConfig{
			Community: 0.3,
			Relevance: 0.4,
			Novelty:   0.3,
		},
		APIs: APIConfig{
			LLMProvider: "ollama",
			LLMModel:    "llama3.2",
		},
		Fetch: FetchConfig{
			Concurrency:    5,
			TimeoutSeconds: 30,
			UserAgent:      "blogmon/1.0",
		},
		Daemon: DaemonConfig{
			IntervalHours: 6,
		},
		Reddit: RedditConfig{
			Subreddits: []string{"programming", "golang"},
		},
	}
}

func Dir() string {
	if dir := os.Getenv("BLOGMON_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".blogmon")
}

func DBPath() string {
	return filepath.Join(Dir(), "blogmon.db")
}

func configPath() string {
	return filepath.Join(Dir(), "config.yaml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath(), data, 0644)
}
