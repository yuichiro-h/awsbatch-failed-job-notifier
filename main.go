package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/yuichiro-h/awsbatch-failed-job-notifier/config"
	"github.com/yuichiro-h/awsbatch-failed-job-notifier/log"
	"go.uber.org/zap"
)

func main() {
	app := cli.NewApp()
	app.Name = "awsbatch-failed-job-notifier"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "config",
		},
	}
	app.Before = func(ctx *cli.Context) error {
		configFilename := ctx.String("config")
		if err := config.Load(configFilename); err != nil {
			return err
		}

		return nil
	}
	app.Action = func(ctx *cli.Context) error {
		if err := execute(); err != nil {
			log.Get().Error("error occurred", zap.String("cause", fmt.Sprintf("%+v", err)))
		}
		return nil
	}
	app.Run(os.Args)
}

func execute() error {
	region := aws.NewConfig().WithRegion(config.Get().AWS.Region)
	sess, err := session.NewSession(region)
	if err != nil {
		return errors.WithStack(err)
	}
	sqsCli := sqs.New(sess)

	var events []event
	var sqsReceiptHandles []*string
	for {
		receiveMessageOut, err := sqsCli.ReceiveMessage(&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: aws.Int64(10),
			QueueUrl:            aws.String(config.Get().AWS.EventSqsURL),
		})
		if err != nil {
			return errors.WithStack(err)
		}
		if len(receiveMessageOut.Messages) == 0 {
			log.Get().Debug("not found messages", zap.String("queue_url", config.Get().AWS.EventSqsURL))
			break
		}

		for _, msg := range receiveMessageOut.Messages {
			var e event
			if err := json.Unmarshal([]byte(*msg.Body), &e); err != nil {
				return errors.WithStack(err)
			}
			events = append(events, e)

			sqsReceiptHandles = append(sqsReceiptHandles, msg.ReceiptHandle)
		}
	}
	log.Get().Info("found events", zap.Int("count", len(events)))

	var notifyInputs []notifyInput
	for _, e := range events {
		queueName := strings.Split(e.Detail.JobQueue, "/")[1]

		channel := config.Get().Slack.DefaultChannel
		for _, q := range config.Get().AWS.JobQueue {
			if q.SlackChannel != nil && glob.MustCompile(q.Name).Match(queueName) {
				channel = q.SlackChannel
			}
		}
		if channel == nil {
			continue
		}

		lastJob := e.Detail.Attempts[len(e.Detail.Attempts)-1]
		jobURL := fmt.Sprintf("https://%[1]s.console.aws.amazon.com"+
			"/batch/home?region=%[1]s#/jobs/queue/arn:aws:batch:%[1]s:%[2]s:job-queue~2F%[3]s/job/%[4]s?state=FAILED",
			e.Region, e.Account, queueName, e.Detail.JobID)

		failedJob := notifyFailedJob{
			QueueName: queueName,
			JobName:   e.Detail.JobName,
			JobURL:    jobURL,
			Reason:    lastJob.StatusReason,
			ExitCode:  lastJob.Container.ExitCode,
		}

		newChannel := true
		for i, n := range notifyInputs {
			if n.Channel == *channel {
				notifyInputs[i].FailedJobs = append(notifyInputs[i].FailedJobs, failedJob)
				newChannel = false
				break
			}
		}
		if newChannel {
			notifyInputs = append(notifyInputs, notifyInput{
				Channel:    *channel,
				FailedJobs: []notifyFailedJob{failedJob},
			})
		}
	}

	for _, n := range notifyInputs {
		if err := notify(&n); err != nil {
			return errors.WithStack(err)
		}
	}

	for _, h := range sqsReceiptHandles {
		_, err = sqsCli.DeleteMessage(&sqs.DeleteMessageInput{
			QueueUrl:      aws.String(config.Get().AWS.EventSqsURL),
			ReceiptHandle: h,
		})
		if err != nil {
			log.Get().Error(err.Error())
			continue
		}
	}

	return nil
}

type event struct {
	Account string `json:"account"`
	Region  string `json:"region"`
	Detail  struct {
		JobName       string `json:"jobName"`
		JobID         string `json:"jobId"`
		JobQueue      string `json:"jobQueue"`
		JobDefinition string `json:"jobDefinition"`
		CreatedAt     int64  `json:"createdAt"`
		Attempts      []struct {
			Container struct {
				ExitCode      int    `json:"exitCode"`
				LogStreamName string `json:"logStreamName"`
			} `json:"container"`
			StartedAt    int64  `json:"startedAt"`
			StoppedAt    int64  `json:"stoppedAt"`
			StatusReason string `json:"statusReason"`
		} `json:"attempts"`
		RetryStrategy struct {
			Attempts int `json:"attempts"`
		} `json:"retryStrategy"`
	} `json:"detail"`
}
