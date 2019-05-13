package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gobwas/glob"
	"github.com/nlopes/slack"
	"github.com/yuichiro-h/awsbatch-failed-job-notifier/config"
	"github.com/yuichiro-h/awsbatch-failed-job-notifier/log"
	"github.com/yuichiro-h/go/aws/sqsrouter"
	"go.uber.org/zap"
)

func main() {
	if err := config.Load(os.Getenv("CONFIG_PATH")); err != nil {
		panic(err)
		return
	}
	log.SetConfig(log.Config{
		Debug: config.Get().Debug,
	})

	region := aws.NewConfig().WithRegion(config.Get().Region)
	sess, err := session.NewSession(region)
	if err != nil {
		log.Get().Error(err.Error())
		return
	}
	r := sqsrouter.New(sess, sqsrouter.WithLogger(log.Get()))
	r.AddHandler(config.Get().EventSqsURL, execute)
	r.Start()

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	r.Stop()
}

func execute(ctx *sqsrouter.Context) {
	var e event
	if err := json.Unmarshal([]byte(*ctx.Message.Body), &e); err != nil {
		log.Get().Error(err.Error())
		return
	}

	queueName := strings.Split(e.Detail.JobQueue, "/")[1]

	slackConfig := config.Get().Slack
	for _, q := range config.Get().JobQueues {
		if glob.MustCompile(q.Name).Match(queueName) {
			slackConfig.Merge(q.Slack)
			break
		}
	}
	if len(e.Detail.Attempts) == 0 {
		ctx.SetDeleteOnFinish(true)
		log.Get().Info("not found event detail", zap.String("message", *ctx.Message.Body))
		return
	}

	lastJob := e.Detail.Attempts[len(e.Detail.Attempts)-1]
	jobURL := fmt.Sprintf("https://%[1]s.console.aws.amazon.com"+
		"/batch/home?region=%[1]s#/jobs/queue/arn:aws:batch:%[1]s:%[2]s:job-queue~2F%[3]s/job/%[4]s?state=FAILED",
		e.Region, e.Account, queueName, e.Detail.JobID)

	_, _, err := slack.New(slackConfig.ApiToken).PostMessage(slackConfig.Channel,
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
			Username: slackConfig.Username,
			IconURL:  slackConfig.IconURL,
		}),
		slack.MsgOptionAttachments(slack.Attachment{
			Color:     slackConfig.AttachmentColor,
			Pretext:   "Found failed jobs",
			Title:     fmt.Sprintf("%s/%s", queueName, e.Detail.JobName),
			TitleLink: jobURL,
			Text:      fmt.Sprintf("%s(%d)", lastJob.StatusReason, lastJob.Container.ExitCode),
		}))
	if err != nil {
		log.Get().Error(err.Error())
		return
	}

	ctx.SetDeleteOnFinish(true)
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
