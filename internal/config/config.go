package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultListenAddr    = ":9734"
	defaultScrapeTimeout = 10 * time.Second
	defaultAPIVersion    = "2.0"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar")
	}

	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	ListenAddr    string   `yaml:"listen_addr"`
	ScrapeTimeout Duration `yaml:"scrape_timeout"`
	APIVersion    string   `yaml:"api_version"`
	Targets       []Target `yaml:"targets"`
}

type Target struct {
	Name               string `yaml:"name"`
	BaseURL            string `yaml:"base_url"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = defaultListenAddr
	}
	if c.ScrapeTimeout.Duration == 0 {
		c.ScrapeTimeout.Duration = defaultScrapeTimeout
	}
	if c.APIVersion == "" {
		c.APIVersion = defaultAPIVersion
	}
}

func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.ListenAddr) == "" {
		errs = append(errs, errors.New("listen_addr is required"))
	}
	if c.ScrapeTimeout.Duration <= 0 {
		errs = append(errs, errors.New("scrape_timeout must be positive"))
	}
	if strings.TrimSpace(c.APIVersion) == "" {
		errs = append(errs, errors.New("api_version is required"))
	}
	if len(c.Targets) == 0 {
		errs = append(errs, errors.New("at least one target is required"))
	}

	seen := make(map[string]struct{}, len(c.Targets))
	for i, target := range c.Targets {
		prefix := fmt.Sprintf("targets[%d]", i)
		if strings.TrimSpace(target.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
		} else if _, ok := seen[target.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate target name %q", target.Name))
		} else {
			seen[target.Name] = struct{}{}
		}
		if strings.TrimSpace(target.BaseURL) == "" {
			errs = append(errs, fmt.Errorf("%s.base_url is required", prefix))
		} else if err := validateBaseURL(target.BaseURL); err != nil {
			errs = append(errs, fmt.Errorf("%s.base_url: %w", prefix, err))
		}
		if strings.TrimSpace(target.Username) == "" {
			errs = append(errs, fmt.Errorf("%s.username is required", prefix))
		}
		if target.Password == "" {
			errs = append(errs, fmt.Errorf("%s.password is required", prefix))
		}
	}

	return errors.Join(errs...)
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}
