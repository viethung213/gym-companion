package admin

import (
	"errors"

	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/discovery"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
)

var _ Inspector = (*Adapter)(nil)

// ConfigSource đại diện cho nguồn cung cấp cấu hình.
type ConfigSource interface {
	GetConfig() *config.Config
}

// ServiceRegistry đại diện cho nguồn quản lý đăng ký service động.
type ServiceRegistry interface {
	RegisterService(serviceID, upstreamURL string, prefixes []string) error
	DeregisterService(serviceID, upstreamURL string) error
	GetDiscoveryInstances() []discovery.Instance
}

// Adapter chuyển đổi telemetry, config và discovery thành Inspector cho Admin Server.
type Adapter struct {
	telemetry *telemetry.Telemetry
	configSrc ConfigSource
	registry  ServiceRegistry
}

// NewAdapter khởi tạo Adapter độc lập.
func NewAdapter(t *telemetry.Telemetry, cfgSrc ConfigSource, registries ...ServiceRegistry) *Adapter {
	var reg ServiceRegistry
	if len(registries) > 0 {
		reg = registries[0]
	}
	return &Adapter{
		telemetry: t,
		configSrc: cfgSrc,
		registry:  reg,
	}
}

func (a *Adapter) GetOverview() telemetry.OverviewStats {
	if a.telemetry == nil || a.telemetry.Metrics == nil {
		return telemetry.OverviewStats{}
	}
	return a.telemetry.Metrics.GetOverview()
}

func (a *Adapter) GetUpstreamStats() []telemetry.UpstreamStats {
	if a.telemetry == nil || a.telemetry.Metrics == nil {
		return nil
	}
	return a.telemetry.Metrics.GetUpstreamStats()
}

func (a *Adapter) QueryTraces(filter telemetry.TraceFilter) []telemetry.RequestJourney {
	if a.telemetry == nil || a.telemetry.Traces == nil {
		return nil
	}
	return a.telemetry.Traces.Query(filter)
}

func (a *Adapter) FormatPrometheus() string {
	if a.telemetry == nil {
		return ""
	}
	return a.telemetry.FormatPrometheus()
}

func (a *Adapter) GetConfig() *config.Config {
	if a.configSrc == nil {
		return nil
	}
	return a.configSrc.GetConfig()
}

func (a *Adapter) RegisterService(serviceID, upstreamURL string, prefixes []string) error {
	if a.registry == nil {
		return errors.New("service registry not configured")
	}
	return a.registry.RegisterService(serviceID, upstreamURL, prefixes)
}

func (a *Adapter) DeregisterService(serviceID, upstreamURL string) error {
	if a.registry == nil {
		return errors.New("service registry not configured")
	}
	return a.registry.DeregisterService(serviceID, upstreamURL)
}

func (a *Adapter) GetDiscoveryInstances() []discovery.Instance {
	if a.registry == nil {
		return nil
	}
	return a.registry.GetDiscoveryInstances()
}
