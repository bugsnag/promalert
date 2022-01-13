package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/bugsnag/microkit/clog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

const MAX_TEXT_LENGTH = 2000

func SlackSendAlertMessage(token, channel string, messageOptions ...slack.MsgOption) (string, string, error) {
	api := slack.New(token, slack.OptionDebug(viper.GetBool("debug")))
	respChannel, respTimestamp, err := api.PostMessage(channel, messageOptions...)
	return respChannel, respTimestamp, err
}

func SlackUpdateAlertMessage(token, channel, timestamp string, messageOptions ...slack.MsgOption) (string, string, error) {
	api := slack.New(token)
	respChannel, respTimestamp, respText, err := api.UpdateMessage(channel, timestamp, messageOptions...)

	_ = respText

	return respChannel, respTimestamp, err
}

func ComposeResolveUpdateBody(alert Alert, headerTemplate string, images ...SlackImage) ([]slack.Block, error) {
	headerTpl, e := ParseTemplate(headerTemplate, alert)
	if e != nil {
		return nil, e
	}
	statusBlock := slack.NewTextBlockObject(
		"mrkdwn",
		truncateText(headerTpl.String(), MAX_TEXT_LENGTH),
		false,
		false,
	)

	var blocks []slack.Block
	blocks = append(blocks, slack.NewSectionBlock(statusBlock, nil, nil))
	for _, image := range images {
		textBlock := slack.NewTextBlockObject("plain_text", truncateText(image.Title, MAX_TEXT_LENGTH), false, false)
		imageAltText := "metric graph " + image.Title
		blocks = append(blocks, slack.NewImageBlock(image.Url, truncateText(imageAltText, MAX_TEXT_LENGTH), "", textBlock))
	}

	return blocks, nil
}

func ComposeUpdateFooter(alert Alert, footerTemplate string) ([]slack.Block, error) {
	footerTpl, e := ParseTemplate(footerTemplate, alert)
	if e != nil {
		return nil, e
	}
	footerBlock := slack.NewTextBlockObject(
		"mrkdwn",
		truncateText(footerTpl.String(), MAX_TEXT_LENGTH),
		false,
		false,
	)

	var blocks []slack.Block
	blocks = append(blocks, slack.NewContextBlock("", footerBlock))

	return blocks, nil
}

func ComposeMessageBody(alert Alert, messageTemplate, headerTemplate string, images ...SlackImage) ([]slack.Block, error) {
	tpl, e := ParseTemplate(messageTemplate, alert)
	if e != nil {
		return nil, e
	}
	headerTpl, e := ParseTemplate(headerTemplate, alert)
	if e != nil {
		return nil, e
	}
	statusBlock := slack.NewTextBlockObject(
		"mrkdwn",
		truncateText(headerTpl.String(), MAX_TEXT_LENGTH),
		false,
		false,
	)

	textBlockObj := slack.NewTextBlockObject(
		"mrkdwn",
		truncateText(tpl.String(), MAX_TEXT_LENGTH),
		false,
		false,
	)
	var blocks []slack.Block
	blocks = append(blocks, slack.NewSectionBlock(statusBlock, nil, nil))
	blocks = append(blocks, slack.NewSectionBlock(textBlockObj, nil, nil))
	for _, image := range images {
		textBlock := slack.NewTextBlockObject("plain_text", truncateText(image.Title, MAX_TEXT_LENGTH), false, false)
		imageAltText := "metric graph " + image.Title
		blocks = append(blocks, slack.NewImageBlock(image.Url, truncateText(imageAltText, MAX_TEXT_LENGTH), "", textBlock))
	}

	return blocks, nil
}

func ParseTemplate(messageTemplate string, alert Alert) (bytes.Buffer, error) {
	funcMap := template.FuncMap{
		"toUpper": strings.ToUpper,
		"now":     time.Now,
		"dateFormat": func(layout string, v interface{}) (string, error) {
			t, err := cast.ToTimeE(v)
			if err != nil {
				return "", err
			}

			return t.Format(layout), nil
		},
	}
	msgTpl, err := template.New("message").Funcs(funcMap).Parse(messageTemplate)

	if err != nil {
		clog.Errorf("error in template: %s", err.Error())
		_ = bugsnag.Notify(errors.Wrap(err, "error in template"))
		return bytes.Buffer{}, err
	}

	var tpl bytes.Buffer
	if err := msgTpl.Execute(&tpl, alert); err != nil {
		return bytes.Buffer{}, err
	}

	return tpl, err
}

func truncateText(text string, maxLength int) string {
	truncateText := "[TRUNCATED]"
	if len(text) > maxLength {
		truncatedLength := MAX_TEXT_LENGTH - len(truncateText)
		return fmt.Sprintf("%s%s", text[:truncatedLength], truncateText)
	}
	return text
}
