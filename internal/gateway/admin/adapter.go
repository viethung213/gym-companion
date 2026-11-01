package admin

import (
	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
)

var _ Inspector = (*Adapter)(nil)

// ConfigSource đại diện cho nguồn cung cấp cấu hình.
type ConfigSource interface {
	GetConfig() *config.Config
}

// Adapter chuyển đổi telemetry và config thành Inspector cho Admin Server.
type Adapter struct {
	telemetry *telemetry.Telemetry
	configSrc ConfigSource
}

// NewAdapter khởi tạo Adapter độc lập.
func NewAdapter(t *telemetry.Telemetry, cfgSrc ConfigSource) *Adapter {
	return &Adapter{
		telemetry: t,
		configSrc: cfgSrc,
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
