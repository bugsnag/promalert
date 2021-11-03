package main

import (
	"net/http/httputil"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/bugsnag/microkit/clog"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func healthz(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "ok",
	})
}

func webhook(c *gin.Context) {
	if viper.GetBool("debug") {
		// Save a copy of this request for debugging.
		requestDump, err := httputil.DumpRequest(c.Request, true)
		if err != nil {
			err = errors.Wrap(err, "Error dumping request")
			_ = bugsnag.Notify(err)
			clog.Error(err)
		}
		clog.Debug("New request: %s", string(requestDump))
	}

	var m HookMessage
	if c.ShouldBindJSON(&m) == nil {
		clog.Info("Alerts: GroupLabels=%v, CommonLabels=%v", m.GroupLabels, m.CommonLabels)

		for _, alert := range m.Alerts {
			if prevAlert, founded := FindAlert(alert); founded {
				alert.Channel = prevAlert.Channel
				alert.MessageTS = prevAlert.MessageTS
				alert.MessageBody = prevAlert.MessageBody
				respChannel, respTimestamp, _, err := alert.PostMessage()
				if err != nil {
					c.String(500, "%v", err)
					return
				}

				if alert.Status == AlertStatusFiring {
					alert.MessageTS = respTimestamp
					alert.Channel = respChannel
					AddAlert(alert)
				}

				clog.Info("Slack update sended, channel: %s thread: %s", respChannel, respTimestamp)
			} else {
				// override channel if specified in rule
				if m.CommonLabels["channel"] != "" {
					alert.Channel = m.CommonLabels["channel"]
				}
				// post new message
				respChannel, respTimestamp, messageBody, err := alert.PostMessage()
				if err != nil {
					c.String(500, "%v", err)
					return
				}

				alert.MessageTS = respTimestamp
				alert.Channel = respChannel
				alert.MessageBody = messageBody

				AddAlert(alert)
			}
		}

		c.JSON(200, map[string]string{"success": "true"})
		return
	}

	c.JSON(400, map[string]string{"status": "Invalid body of request"})
}
