package config

import (
	"github.com/BurntSushi/toml"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/reporter"
)

type Config struct {
	Presets []Preset
	smtp    *SMTPReporter
	webhook *WebhookReporter
}

func LoadConfiguration(p string) (*Config, error) {
	var config Config

	if _, err := toml.DecodeFile(p, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c Config) GetReporters() reporter.ChainReporter {
	chain := reporter.ChainReporter{}
	chain.Add(reporter.StdOut{})

	if c.webhook != nil {
		chain.Add(reporter.NewWebhookReporter(c.webhook.Url))
	}

	if c.smtp != nil {
		chain.Add(reporter.NewSmtpReporter(c.smtp.Username, c.smtp.Password, c.smtp.Host, c.smtp.RecipientList))
	}

	return chain
}

type SMTPReporter struct {
	Username      string   `toml:"username"`
	Password      string   `toml:"password"`
	Host          string   `toml:"host"`
	RecipientList []string `toml:"recipients"`
}

type WebhookReporter struct {
	Url string `toml:"url"`
}

type Preset struct {
	paths    []string
	clamav   *string
	database *string
}
