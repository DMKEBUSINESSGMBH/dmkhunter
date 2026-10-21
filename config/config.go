package config

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/reporter"
	"github.com/pelletier/go-toml/v2"
	"os"
)

type Config struct {
	Presets []Preset
	Smtp    *SMTPReporter
	Webhook *WebhookReporter
}

type Preset struct {
	Paths    []string `toml:"paths"`
	Ignores  []string `toml:"ignores"`
	Clamav   *string  `toml:"clamav"`
	Database *string  `toml:"database"`
}

type SMTPReporter struct {
	Username      string   `toml:"username"`
	Password      string   `toml:"password"`
	Host          string   `toml:"host"`
	RecipientList []string `toml:"recipients"`
	FromAddress   *string  `toml:"from-address"`
	Topic         *string  `toml:"topic"`
}

type WebhookReporter struct {
	Url string `toml:"url"`
}

func LoadConfiguration(p string) (*Config, error) {
	var config Config

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := toml.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c Config) GetReporters() reporter.ChainReporter {
	chain := reporter.ChainReporter{}
	chain.Add(reporter.StdOut{})

	if c.Webhook != nil {
		chain.Add(reporter.NewWebhookReporter(c.Webhook.Url))
	}

	if c.Smtp != nil {
		topicname := "dmkhunter"
		if c.Smtp.Topic != nil {
			topicname = *c.Smtp.Topic
		} else {
			if osHost, err := os.Hostname(); err == nil {
				topicname = osHost
			}
		}
		smtpFrom := "dmkhunter@" + topicname
		if c.Smtp.FromAddress != nil {
			smtpFrom = *c.Smtp.FromAddress
		}

		chain.Add(reporter.NewSmtpReporter(c.Smtp.Username, c.Smtp.Password, c.Smtp.Host, c.Smtp.RecipientList, smtpFrom, topicname))
	}

	return chain
}

func (c Config) GetPaths() []string {
	var paths []string

	for _, preset := range c.Presets {
		paths = append(paths, preset.Paths...)
	}

	return paths
}

// GetDatabases returns the database files of all presets, without duplicates.
func (c Config) GetDatabases() []string {
	var databases []string
	seen := map[string]bool{}

	for _, preset := range c.Presets {
		name := preset.GetDatabase()

		if !seen[name] {
			seen[name] = true
			databases = append(databases, name)
		}
	}

	return databases
}

// GetDatabase returns the database file of the preset, rescan writes to hunter.db if none is configured.
func (p Preset) GetDatabase() string {
	if p.Database != nil {
		return *p.Database
	}

	return "hunter.db"
}
