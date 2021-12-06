package main

import (
	"net/url"
	"strconv"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/bugsnag/microkit/clog"
	"github.com/mitchellh/hashstructure"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"
)

func (alert Alert) Hash() string {
	hash, err := hashstructure.Hash(map[string]KV{
		"labels": alert.Labels,
	}, nil)
	if err != nil {
		err = errors.Wrap(err, "Hash cant be calculated")
		_ = bugsnag.Notify(err)
	}
	clog.Infof("Hash calculated: %d", hash)

	return strconv.FormatUint(hash, 10)
}

func (alert Alert) GeneratePictures(generatorQuery url.Values) ([]SlackImage, error) {
	var alertFormula string
	for key, param := range generatorQuery {
		if key == "g0.expr" {
			alertFormula = param[0]
			break
		}
	}

	plotExpression := GetPlotExpr(alertFormula)
	queryTime, duration := alert.GetPlotTimeRange()

	var images []SlackImage

	for _, expr := range plotExpression {
		plot, err := Plot(
			expr,
			queryTime,
			duration,
			time.Duration(viper.GetInt64("metric_resolution")),
			viper.GetString("prometheus_url"),
			alert,
		)
		if err != nil {
			return nil, errors.Wrap(err, "Plotter error")
		}

		publicURL, err := UploadFile(viper.GetString("s3_bucket"), viper.GetString("s3_region"), plot)
		if err != nil {
			return nil, errors.Wrap(err, "S3 error")
		}
		clog.Infof("Graph uploaded, URL: %s", publicURL)

		images = append(images, SlackImage{
			Url:   publicURL,
			Title: expr.String(),
		})
	}

	return images, nil
}

func (alert Alert) PostMessage(generatorQuery url.Values) error {
	clog.Warnf("Alert: channel=%s,status=%s,Labels=%v,Annotations=%v", alert.Channel, alert.Status, alert.Labels, alert.Annotations)
	severity := alert.Labels["severity"]
	options := make([]slack.MsgOption, 0)

	attachment := slack.Attachment{}
	attachment.Blocks.BlockSet = make([]slack.Block, 0)
	// palette: https://bugsnag-component-library.netlify.app/?path=/docs/docs-colors--page
	switch severity {
	case "warn":
		attachment.Color = "#ffa300" // sunflower
	case "critical":
		attachment.Color = "#ff5a60" // coral
	case "page":
		attachment.Color = "#a15fff" // orchid
	}

	if alert.Status == AlertStatusFiring {
		clog.Info("Composing full message")
		images, err := alert.GeneratePictures(generatorQuery)
		if err != nil {
			return err
		}

		messageBlocks, err := ComposeMessageBody(
			alert,
			viper.GetString("message_template"),
			viper.GetString("header_template"),
			images...,
		)
		if err != nil {
			return err
		}

		alert.MessageBody = messageBlocks
		attachment.Blocks.BlockSet = append(attachment.Blocks.BlockSet, messageBlocks...)

		if alert.MessageTS != "" {
			options = append(options, slack.MsgOptionBroadcast())
			clog.Info("Adding broadcast flag to message")
		}
	} else {
		clog.Info("Composing short update message")
		attachment.Color = "#8cc63f" // green

		images, err := alert.GeneratePictures(generatorQuery)
		if err != nil {
			return err
		}

		messageBlocks, err := ComposeResolveUpdateBody(
			alert,
			viper.GetString("header_template"),
			images...,
		)
		if err != nil {
			return err
		}

		options = append(options, slack.MsgOptionBroadcast())
		attachment.Blocks.BlockSet = append(attachment.Blocks.BlockSet, messageBlocks...)
	}

	channel := viper.GetString("slack_channel")

	if alert.Channel != "" {
		channel = alert.Channel
	}

	options = append(options, slack.MsgOptionAttachments(attachment))
	respChannel, respTimestamp, err := SlackSendAlertMessage(
		viper.GetString("slack_token"),
		channel,
		options...,
	)
	if err != nil {
		return err
	}

	clog.Infof("Slack message sent, channel: %s timestamp: %s", respChannel, respTimestamp)
	return nil
}

func (alert Alert) GetPlotTimeRange() (time.Time, time.Duration) {
	var queryTime time.Time
	var duration time.Duration
	if alert.StartsAt.Second() > alert.EndsAt.Second() {
		queryTime = alert.StartsAt
		duration = time.Minute * 20
	} else {
		queryTime = alert.EndsAt
		duration = queryTime.Sub(alert.StartsAt)

		if duration < time.Minute*20 {
			duration = time.Minute * 20
		}
	}
	clog.Infof("Querying Time %v Duration: %v", queryTime, duration)
	return queryTime, duration
}
