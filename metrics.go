package main

// Metrics fetches data from Prometheus.
import (
	"context"
	"time"

	"github.com/pkg/errors"
	prometheus "github.com/prometheus/client_golang/api"
	prometheusApi "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func Metrics(server, query string, queryTime time.Time, duration, step time.Duration) (model.Matrix, error) {
	client, err := prometheus.NewClient(prometheus.Config{Address: server})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Prometheus client")
	}

	api := prometheusApi.NewAPI(client)
	value, _, err := api.QueryRange(context.Background(), query, prometheusApi.Range{
		Start: queryTime.Add(-duration),
		End:   queryTime,
		Step:  duration / step,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query Prometheus")
	}

	metrics, ok := value.(model.Matrix)
	if !ok {
		return nil, errors.Wrap(err, "unsupported result format")
	}

	return metrics, nil
}
