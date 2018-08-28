package config

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var c Config

type Config struct {
	Debug bool

	Slack struct {
		APIToken        string  `yaml:"api_token"`
		Username        string  `yaml:"username"`
		IconURL         string  `yaml:"icon_url"`
		AttachmentColor string  `yaml:"attachment_color"`
		DefaultChannel  *string `yaml:"default_channel"`
	} `yaml:"slack"`

	AWS struct {
		Region      string `yaml:"region"`
		EventSqsURL string `yaml:"event_sqs_url"`
		JobQueue    []struct {
			Name         string  `yaml:"name"`
			SlackChannel *string `yaml:"slack_channel"`
		} `yaml:"job_queue"`
	} `yaml:"aws"`
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
