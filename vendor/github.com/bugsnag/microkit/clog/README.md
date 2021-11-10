# clog

Provides a global logging library to record `INFO`, `WARN` and `ERROR` logs. The library can store logging metadata in the context which is used to enrich log messages. The metadata is stored by creating a new copy of the context and adding new fields, inline with how values are added to context ([see Context with Values](https://levelup.gitconnected.com/context-in-golang-98908f042a57) for more details on context). The logger uses [uber/zap](https://github.com/uber-go/zap) logging library and defaults to writing to stdout in color. Examples of logs are

```console
2020-09-13T20:44:33Z    INFO    Test Info      {"yesterday": "2020-09-13T20:44:33Z", "period": "5s", "day": "mon", "valid": true}
2020-09-13T20:44:33Z    WARN    Test Warn      {"yesterday": "2020-09-13T20:44:33Z", "period": "5s", "day": "mon", "valid": true}
2020-09-13T20:44:33Z    ERROR   Test Error     {"yesterday": "2020-09-13T20:44:33Z", "period": "5s", "day": "mon", "valid": true}
```

Logs in production are sent and can be viewed in [Stackdriver](https://console.cloud.google.com/logs/query?project=bugsnag-155907). Normally, we only record `WARN` and `ERROR` logs to reduce the amount the traffic and cost. The rules determining which logs are ingested and which are ignored by Stackdriver can be found and updated in the [infra repo - stackdriver-logging-exclusions.tf](https://github.com/bugsnag/infra/blob/master/terraform-gcp/stackdriver-logging-exclusions.tf) file.

## Usage 

The logger is initialized automatically with default settings when the package is imported (color enabled by default). Logs can be written at the following levels.

- **INFO** - Used for informational messages that highlight the progress of the application at coarse-grained level.
- **WARN** - Used for potentially harmful situations, but are normally handled.
- **ERROR** - Used for error events that may or may not still allow the application to continue running.

Care should be taken to ensure that large numbers of logs are not created. This can make it hard to view important logs and slow down the app. Consider generating aggregate logs (writing a single log for multiple events) to allow the user to see the app is still working.

To write simple logs

```go
// Recording simple string log messages
clog.Info("Test Info")
clog.Warn("Test Warn")
clog.Error("Test Error")

// Recording simple string log messages with format
clog.Infof("Test Info with format: %v", 34)
clog.Warnf("Test Warn with format: %v", "bob")
clog.Errorf("Test Error with format: %v", []string{"hello", "world"})
```

To store metadata in the context to use in logging
```go
ctx := context.Background()

// Add these four fields to the context
ctx = clog.WithField(ctx, "yesterday", time.Now())
ctx = clog.WithField(ctx, "period", 5*time.Second)
ctx = clog.WithField(ctx, "day", "mon")
ctx = clog.WithField(ctx, "valid", true)

// Recording simple string log messages with metadata
clog.Infoc(ctx, "Test Info")
clog.Warnc(ctx, "Test Warn")
clog.Errorc(ctx, "Test Error")

// Recording simple string log messages with format and metadata
clog.Infocf("Test Info with format: %v", 34)
clog.Warncf("Test Warn with format: %v", "bob")
clog.Errorcf("Test Error with format: %v", []string{"hello", "world"})
```

Color output can be enabled or disable. For production, Stackdriver does not support colored logs and should be disabled.

Note that this will replace the logger and is not thread safe, so should be configured at the beginning of the app.

```go
clog.DisableColor()  // Switch logger to non color output
clog.EnableColor()   // Switch logger to color output
```

To ensure that logs are flushed when the app is closed
```go
func main() {
  defer clog.Flush()

  ...
}
```

Some of our libraries can also log and require a logger to be provided. The logger can be passed into these libraries. For example for the rabbit library
```go
logClient := clog.LogClient()
rabbit.SetLogger(logClient)
```