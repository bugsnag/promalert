package main

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"
)

func (alert Alert) Hash() string {
	hash, err := hashstructure.Hash(map[string]KV{
		"labels": alert.Labels,
	}, nil)
	fatal(err, "Hash cant be calculated")
	log.Printf("Hash calculated: %d", hash)

	return strconv.FormatUint(hash, 10)
}

func (alert Alert) GeneratePictures() ([]SlackImage, error) {
	generatorUrl, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return nil, err
	}

	generatorQuery, err := url.ParseQuery(generatorUrl.RawQuery)
	if err != nil {
		return nil, err
	}

	var alertFormula string
	for key, param := range generatorQuery {
		if key == "g0.expr" {
			alertFormula = param[0]
			break
		}
	}
	fmt.Println(alertFormula)

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
			return nil, fmt.Errorf("Plotter error: %v\n", err)
		}

		publicURL, err := UploadFile(viper.GetString("s3_bucket"), viper.GetString("s3_region"), plot)
		if err != nil {
			return nil, fmt.Errorf("S3 error: %v\n", err)
		}
		log.Printf("Graph uploaded, URL: %s", publicURL)

		images = append(images, SlackImage{
			Url:   publicURL,
			Title: expr.String(),
		})
	}

	return images, nil
}

func (alert Alert) PostMessage() (string, string, []slack.Block, error) {
	log.Printf("Alert: status=%s,Labels=%v,Annotations=%v", alert.Status, alert.Labels, alert.Annotations)
	severity := alert.Labels["severity"]
	log.Printf("no action on severity: %s", severity)

	options := make([]slack.MsgOption, 0)

	attachment := slack.Attachment{}
	attachment.Blocks.BlockSet = make([]slack.Block, 0)
	// palette: https://coolors.co/c200fb-ec0868-fc2f00-ec7d10-ffbc0a
	switch severity {
	case "warn":
		attachment.Color = "#ffa300" // yellow
	case "critical":
		attachment.Color = "#ff5a60" // orange
	case "page":
		attachment.Color = "#a15fff" // scarlet
	}

	if alert.Status == AlertStatusFiring || alert.MessageTS == "" {
		log.Print("Composing full message")
		images, err := alert.GeneratePictures()
		if err != nil {
			return "", "", nil, err
		}

		messageBlocks, err := ComposeMessageBody(
			alert,
			viper.GetString("message_template"),
			viper.GetString("header_template"),
			images...,
		)
		if err != nil {
			return "", "", nil, err
		}

		alert.MessageBody = messageBlocks
		attachment.Blocks.BlockSet = append(attachment.Blocks.BlockSet, messageBlocks...)

		if alert.MessageTS != "" {
			options = append(options, slack.MsgOptionBroadcast())
			log.Print("Adding broadcast flag to message")
		}
	} else {
		log.Print("Composing short update message")
		images, err := alert.GeneratePictures()
		if err != nil {
			return "", "", nil, err
		}

		messageBlocks, err := ComposeResolveUpdateBody(
			alert,
			viper.GetString("header_template"),
			images...,
		)
		if err != nil {
			return "", "", nil, err
		}

		options = append(options, slack.MsgOptionBroadcast())
		attachment.Blocks.BlockSet = append(attachment.Blocks.BlockSet, messageBlocks...)
		attachment.Color = "#8cc63f"
	}

	if alert.MessageTS != "" {
		log.Printf("MessageTS found, posting to thread: %s", alert.MessageTS)
		options = append(options, slack.MsgOptionTS(alert.MessageTS))

		d, err := ComposeUpdateFooter(alert, viper.GetString("footer_template"))
		if err != nil {
			return "", "", nil, err
		}

		updateAttachment := slack.Attachment{}
		updateAttachment.Blocks.BlockSet = append(alert.MessageBody, d...)
		switch severity {
		case "warn":
			attachment.Color = "#ffa300" // yellow
		case "critical":
			attachment.Color = "#ff5a60" // orange
		case "page":
			attachment.Color = "#a15fff" // scarlet
		}

		respChannel, respTimestamp, err := SlackUpdateAlertMessage(
			viper.GetString("slack_token"),
			alert.Channel,
			alert.MessageTS,
			slack.MsgOptionAttachments(updateAttachment),
		)
		if err != nil {
			return "", "", nil, err
		}

		log.Printf("Slack message updated, channel: %s thread: %s", respChannel, respTimestamp)
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
		return "", "", nil, err
	}

	log.Printf("Slack message sent, channel: %s thread: %s", respChannel, respTimestamp)

	return respChannel, respTimestamp, alert.MessageBody, nil
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
	log.Printf("Querying Time %v Duration: %v", queryTime, duration)
	return queryTime, duration
}
