package metrics

import (
	"github.com/DataDog/datadog-go/v5/statsd"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
)

// New builds a DogStatsD client pointed at the Datadog Agent (DD_AGENT_HOST,
// UDP 8125 — see prod/datadog.tf's dogstatsd.useHostPort). Returns a no-op
// client when cfg.Disabled (DD_AGENT_HOST unset, e.g. local/dev), so callers
// never need a nil check.
func New(cfg *config.DogStatsDConfig) (statsd.ClientInterface, error) {
	if cfg.Disabled {
		return &statsd.NoOpClient{}, nil
	}
	return statsd.New(cfg.Addr)
}
