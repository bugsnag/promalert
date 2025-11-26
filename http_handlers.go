package main

import (
	"net/http/httputil"
	"net/url"

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
	ctx := c.Request.Context()
	if viper.GetBool("debug") {
		// Save a copy of this request for debugging.
		requestDump, err := httputil.DumpRequest(c.Request, true)
		if err != nil {
			err = errors.Wrap(err, "Error dumping request")
			_ = bugsnag.Notify(err, ctx)
			clog.Error(err.Error())
		}
		clog.Infof("New request: %s", string(requestDump))
	}

	var m HookMessage
	if c.ShouldBindJSON(&m) == nil {
		clog.Infof("Alerts: GroupLabels=%v, CommonLabels=%v", m.GroupLabels, m.CommonLabels)

		for _, alert := range m.Alerts {
			alertName := alert.Labels["alertname"]
			// shorten all alert annotation URLs
			cli := NewLinksClient()
			for k, txt := range alert.Annotations {
				n, err := cli.ReplaceLinks(ctx, txt)
				if err != nil {
					e := errors.Wrap(err, "Error whilst shortening links")
					_ = bugsnag.Notify(e, ctx,
						bugsnag.MetaData{
							"Alert": {
								"Name": alertName,
							},
							"Kutt": {
								"URL":       n,
								"URLLength": len(n),
							},
						})
					clog.Error(e.Error())
				}
				alert.Annotations[k] = n
			}

			// get chart url
			generatorUrl, err := url.Parse(alert.GeneratorURL)
			if err != nil {
				err = errors.Wrap(err, "Could not parse generator url")
				_ = bugsnag.Notify(err, ctx,
					bugsnag.MetaData{
						"Alert": {
							"Name":         alertName,
							"GeneratorURL": alert.GeneratorURL,
						},
					})
				clog.Error(err.Error())
			}

			// from the chart url get the expressions to build the charts
			generatorQuery, err := url.ParseQuery(generatorUrl.RawQuery)
			if err != nil {
				err = errors.Wrap(err, "Could not parse query from generator url")
				_ = bugsnag.Notify(err, ctx,
					bugsnag.MetaData{
						"Alert": {
							"Name":     alertName,
							"RawQuery": generatorUrl.RawQuery,
						},
					})
				clog.Error(err.Error())
			}

			// shorten generator URL
			n, err := cli.ReplaceLinks(ctx, alert.GeneratorURL)
			if err != nil {
				e := errors.Wrap(err, "Error shortening generator URL")
				_ = bugsnag.Notify(e, ctx,
					bugsnag.MetaData{
						"Alert": {
							"Name": alertName,
						},
						"Kutt": {
							"URL": n,
						},
					})
				clog.Error(e.Error())
			}
			alert.GeneratorURL = n

			// override channel if specified in rule
			if m.CommonLabels["channel"] != "" {
				alert.Channel = m.CommonLabels["channel"]
			}

			// post new message
			err = alert.PostMessage(generatorQuery)
			if err != nil {
				c.String(500, "%v", err)
				err = errors.Wrap(err, "Error posting Slack message")
				_ = bugsnag.Notify(err, ctx,
					bugsnag.MetaData{
						"Alert": {
							"Name": alertName,
						},
						"Slack": {
							"Query": generatorQuery,
						},
					})
				clog.Error(err.Error())
				return
			}
		}

		c.JSON(200, map[string]string{"success": "true"})
		return
	}

	c.JSON(400, map[string]string{"status": "Invalid body of request"})
}
