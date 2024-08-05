package main

import (
	bugsnaggin "github.com/bugsnag/bugsnag-go/gin"
	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/font/liberation"
)

var fontCache *font.Cache

func main() {
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")
	viper.AddConfigPath("/etc/promalert")
	viper.SetConfigName("config")

	err := viper.ReadInConfig()
	if err != nil {
		err = errors.Wrap(err, "Fatal error config file")
		panic(err)
	}
	viper.AutomaticEnv()
	viper.SetDefault("bugsnag_release_stage", "development")
	viper.SetDefault("bugsnag_api_key", "")
	viper.SetEnvPrefix("promalert")

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:          viper.GetString("bugsnag_api_key"),
		ProjectPackages: []string{"main", "github.com/bugsnag/promalert"},
		ReleaseStage:    viper.GetString("bugsnag_release_stage"),
		Synchronous:     true,
	})

	fontCache = font.NewCache(liberation.Collection())

	g := gin.Default()
	g.Use(bugsnaggin.AutoNotify())

	r := gin.New()
	r.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"))
	r.Use(gin.Recovery())

	r.GET("/healthz", healthz)
	r.POST("/webhook", webhook)

	err = r.Run(":" + viper.GetString("http_port"))
	if err != nil {
		err = errors.Wrap(err, "Can't start web server")
		_ = bugsnag.Notify(err)
		panic(err)
	}
}
