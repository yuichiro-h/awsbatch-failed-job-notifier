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

	region := aws.NewConfig().WithRegion(config.Get().AWS.Region)
	sess, err := session.NewSession(region)
	if err != nil {
		log.Get().Error(err.Error())
		return
	}
	r := sqsrouter.New(sess, sqsrouter.WithLogger(log.Get()))
	r.AddHandler(config.Get().AWS.EventSqsURL, execute)
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

	var notifyInputs []notifyInput
	queueName := strings.Split(e.Detail.JobQueue, "/")[1]

	channel := config.Get().Slack.DefaultChannel
	for _, q := range config.Get().AWS.JobQueue {
		if q.SlackChannel != nil && glob.MustCompile(q.Name).Match(queueName) {
			channel = q.SlackChannel
		}
	}
	if channel == nil {
		log.Get().Info("not match channel", zap.String("message", *ctx.Message.Body))
		return
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

	for _, n := range notifyInputs {
		if err := notify(&n); err != nil {
			log.Get().Error(err.Error())
			return
		}
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
