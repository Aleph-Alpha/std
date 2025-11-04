package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	Server      *http.Server
	Registry    *prometheus.Registry
	serviceName string
}

func NewMetrics(cfg Config) *Metrics {
	registry := prometheus.NewRegistry()

	wrappedRegistry := prometheus.WrapRegistererWith(prometheus.Labels{"service": cfg.ServiceName}, registry)

	if cfg.EnableDefaultCollectors {
		wrappedRegistry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewBuildInfoCollector(),
		)
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: handler,
	}

	return &Metrics{
		Server:      server,
		Registry:    registry,
		serviceName: cfg.ServiceName,
	}
}
