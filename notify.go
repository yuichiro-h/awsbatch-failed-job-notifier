package main

import (
	"fmt"

	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"github.com/yuichiro-h/awsbatch-failed-job-notifier/config"
)

type notifyInput struct {
	Channel    string
	FailedJobs []notifyFailedJob
}

type notifyFailedJob struct {
	QueueName string
	JobName   string
	JobURL    string
	Reason    string
	ExitCode  int
}

func notify(in *notifyInput) error {
	var attachments []slack.Attachment
	for _, j := range in.FailedJobs {
		attachments = append(attachments, slack.Attachment{
			Color:     config.Get().Slack.AttachmentColor,
			Title:     fmt.Sprintf("%s/%s", j.QueueName, j.JobName),
			TitleLink: j.JobURL,
			Text:      fmt.Sprintf("%s(%d)", j.Reason, j.ExitCode),
		})
	}

	params := slack.PostMessageParameters{
		Username:    config.Get().Slack.Username,
		IconURL:     config.Get().Slack.IconURL,
		Attachments: attachments,
	}

	_, _, err := slack.New(config.Get().Slack.APIToken).PostMessage(in.Channel, "Found failed jobs", params)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
