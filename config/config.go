package config

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var c Config

type Config struct {
	Debug       bool        `yaml:"debug"`
	Region      string      `yaml:"region"`
	EventSqsURL string      `yaml:"event_sqs_url"`
	Slack       SlackConfig `yaml:"slack"`
	JobQueues   []struct {
		Name  string      `yaml:"name"`
		Slack SlackConfig `yaml:"slack"`
	} `yaml:"job_queues"`
}

type SlackConfig struct {
	ApiToken        string `yaml:"api_token"`
	Username        string `yaml:"username"`
	Channel         string `yaml:"channel"`
	AttachmentColor string `yaml:"attachment_color"`
	IconURL         string `yaml:"icon_url"`
}

func (c *SlackConfig) Merge(sc SlackConfig) {
	if sc.ApiToken != "" {
		c.ApiToken = sc.ApiToken
	}
	if sc.AttachmentColor != "" {
		c.AttachmentColor = sc.AttachmentColor
	}
	if sc.Channel != "" {
		c.Channel = sc.Channel
	}
	if sc.IconURL != "" {
		c.IconURL = sc.IconURL
	}
	if sc.Username != "" {
		c.Username = sc.Username
	}
}

func Load(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := yaml.Unmarshal(data, &c); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func Get() *Config {
	return &c
}
