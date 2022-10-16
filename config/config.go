package config

import (
	"github.com/DMKEBUSINESSGMBH/dmkhunter/reporter"
	"github.com/pelletier/go-toml/v2"
	"log"
	"os"
)

type Config struct {
	Presets []Preset
	Smtp    *SMTPReporter
	Webhook *WebhookReporter
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

type Preset struct {
	Paths    []string `toml:"paths"`
	Ignores  []string `toml:"ignores"`
	Clamav   *string  `toml:"clamav"`
	Database *string  `toml:"database"`
}

func LoadConfiguration(p string) (*Config, error) {
	var config Config

	f, err := os.Open(p)
	if err != nil {
		log.Fatal(err)
	}

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
