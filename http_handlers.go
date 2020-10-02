package main

import (
	"fmt"
	"log"
	"net/http/httputil"

	"github.com/gin-gonic/gin"
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
			fmt.Println(err)
		}
		log.Printf("New request")
		fmt.Println(string(requestDump))
	}

	var m HookMessage
	if c.ShouldBindJSON(&m) == nil {
		log.Printf("Alerts: GroupLabels=%v, CommonLabels=%v", m.GroupLabels, m.CommonLabels)

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

				log.Printf("Slack update sended, channel: %s thread: %s", respChannel, respTimestamp)
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

//
//func webhook(w http.ResponseWriter, r *http.Request) {
//	if viper.GetBool("debug") {
//		// Save a copy of this request for debugging.
//		requestDump, err := httputil.DumpRequest(r, true)
//		if err != nil {
//			fmt.Println(err)
//		}
//		log.Printf("New request")
//		fmt.Println(string(requestDump))
//	}
//
//	dec := json.NewDecoder(r.Body)
//	defer r.Body.Close()
//
//	var m HookMessage
//	if err := dec.Decode(&m); err != nil {
//		log.Printf("error decoding message: %v", err)
//		http.Error(w, "invalid request body", 400)
//		return
//	}
//
//	log.Printf("Alerts: GroupLabels=%v, CommonLabels=%v", m.GroupLabels, m.CommonLabels)
//
//	for _, alert := range m.Alerts {
//		if prevAlert, founded := FindAlert(alert); founded {
//			alert.Channel = prevAlert.Channel
//			alert.MessageTS = prevAlert.MessageTS
//			alert.MessageBody = prevAlert.MessageBody
//			respChannel, respTimestamp, _ := alert.PostMessage()
//			if alert.Status == AlertStatusFiring {
//				alert.MessageTS = respTimestamp
//				alert.Channel = respChannel
//				AddAlert(alert)
//			}
//			log.Printf("Slack update sended, channel: %s thread: %s", respChannel, respTimestamp)
//		} else {
//			// post new message
//			respChannel, respTimestamp, messageBody := alert.PostMessage()
//			alert.MessageTS = respTimestamp
//			alert.Channel = respChannel
//			alert.MessageBody = messageBody
//
//			AddAlert(alert)
//		}
//	}
//
//	w.Header().Set("Content-Type", "application/json")
//	_, err := w.Write([]byte("{\"success\": true}"))
//	fatal(err, "failed to send response")
//}
