package main

import (
	"fmt"

	"github.com/bugsnag/bugsnag-go"
	bugsnaggin "github.com/bugsnag/bugsnag-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")
	viper.AddConfigPath("/etc/promalert")
	viper.SetConfigName("config")

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	viper.AutomaticEnv()
	viper.SetDefault("bugsnag_release_stage", "development")
	viper.SetEnvPrefix("promalert")

	bugsnag.Configure(bugsnag.Configuration{
		APIKey: "a0936975687e3d8abbf8a593dc646f87",
		// The import paths for the Go packages containing your source files
		ProjectPackages: []string{"main", "github.com/bugsnag/promalert"},
		ReleaseStage:    viper.GetString("bugsnag_release_stage"),
		Synchronous:     true,
	})

	g := gin.Default()
	g.Use(bugsnaggin.AutoNotify())

	r := gin.New()
	r.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/healthz"))
	r.Use(gin.Recovery())

	r.GET("/healthz", healthz)
	r.POST("/webhook", webhook)

	err = r.Run(":" + viper.GetString("http_port"))
	panic(fmt.Errorf("Cant start web server: %s \n", err))
}
