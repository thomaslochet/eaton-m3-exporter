package exporter

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"gitlab.synost.net/synost/eaton-m3-exporter/internal/m3"
)

func TestCollectorEmitsSnapshotMetrics(t *testing.T) {
	t.Parallel()

	collector := NewCollector([]Client{mockClient{snapshot: sampleSnapshot()}}, time.Second, slog.Default())
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	assertMetricValue(t, families, "eaton_m3_up", 1)
	assertMetricValue(t, families, "eaton_m3_alarm_active_count", 2)
	assertMetricValue(t, families, "eaton_m3_voltage_volts", 244.6)
	assertMetricValue(t, families, "eaton_m3_temperature_kelvin", 298.15)
	assertMetricValue(t, families, "eaton_m3_environment_input_active", 1)
	assertMetricValue(t, families, "eaton_m3_power_distribution_info", 1)
}

func TestCollectorPartialFailure(t *testing.T) {
	t.Parallel()

	collector := NewCollector([]Client{
		mockClient{target: m3.Target{Name: "ok", BaseURL: "https://ok.example"}, snapshot: sampleSnapshotFor("ok", "https://ok.example")},
		mockClient{target: m3.Target{Name: "bad", BaseURL: "https://bad.example"}, err: errors.New("boom")},
	}, time.Second, slog.Default())
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	if got := metricValueWithLabel(t, families, "eaton_m3_up", "target", "ok"); got != 1 {
		t.Fatalf("ok up = %v, want 1", got)
	}
	if got := metricValueWithLabel(t, families, "eaton_m3_up", "target", "bad"); got != 0 {
		t.Fatalf("bad up = %v, want 0", got)
	}
	if got := metricValueWithLabel(t, families, "eaton_m3_scrape_errors_total", "target", "bad"); got != 1 {
		t.Fatalf("bad errors = %v, want 1", got)
	}
}

func sampleSnapshot() m3.Snapshot {
	return sampleSnapshotFor("ups-main", "https://192.0.2.10")
}

func sampleSnapshotFor(target string, baseURL string) m3.Snapshot {
	return m3.Snapshot{
		TargetName:               target,
		BaseURL:                  baseURL,
		PowerServiceStatus:       m3.Status{Operating: "in service", Health: "ok"},
		EnvironmentServiceStatus: m3.Status{Operating: "in service", Health: "ok"},
		AlarmCounts:              m3.AlarmCounts{ActiveCount: 2, TotalCount: 4},
		MostCriticalAlarm:        m3.Alarm{ID: "6705", State: "open", Code: "80D", Level: "warning", Description: "Internal configuration failure"},
		PowerDistributions: []m3.PowerDistribution{
			{
				ID:             "1",
				Identification: m3.Identification{UUID: "pd-uuid", FriendlyName: "UPS", SerialNumber: "S1"},
				Status:         m3.Status{Operating: "in service", Health: "warning", ConfigurationFault: boolPtr(true)},
				PowerBank:      m3.PowerBankStatus{Operating: "stopped", Health: "ok", Supplied: boolPtr(true)},
				Inputs: []m3.PowerResource{
					{ID: "1", Kind: "input", Identification: m3.Identification{UUID: "input-uuid", FriendlyName: "Main utility"}, Status: m3.Status{Supplied: boolPtr(true)}, Measures: m3.Measures{Voltage: floatPtr(244.6), Current: floatPtr(0.1)}},
				},
			},
		},
		Suppliers: []m3.Supplier{
			{ID: "supplier-1", Measures: m3.Measures{Voltage: floatPtr(243.8)}, Summary: m3.SupplierSummary{CapacityRuntime: 59940, ProtectionCapacityPercent: 100, Mode: "normal", Quality: "protecting"}},
		},
		Temperatures: []m3.EnvironmentSensor{{ID: "temp-1", Name: "Sensor 1", Measure: 298.15, AlarmLevel: "good"}},
		Humidities:   []m3.EnvironmentSensor{{ID: "hum-1", Name: "Sensor 1", Measure: 50, AlarmLevel: "good"}},
		EnvironmentInputs: []m3.EnvironmentInput{
			{ID: "input-1", Name: "Input 1.1", Active: true, AlarmLevel: "critical"},
		},
	}
}

type mockClient struct {
	target   m3.Target
	snapshot m3.Snapshot
	err      error
}

func (m mockClient) Target() m3.Target {
	if m.target.Name != "" {
		return m.target
	}
	return m3.Target{Name: m.snapshot.TargetName, BaseURL: m.snapshot.BaseURL}
}

func (m mockClient) FetchSnapshot(_ context.Context) (m3.Snapshot, error) {
	if m.err != nil {
		return m3.Snapshot{}, m.err
	}
	return m.snapshot, nil
}

func assertMetricValue(t *testing.T, families []*dto.MetricFamily, name string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if valueOf(metric) == want {
				return
			}
		}
		t.Fatalf("metric %s found but no sample value %v", name, want)
	}
	t.Fatalf("metric %s not found", name)
}

func metricValueWithLabel(t *testing.T, families []*dto.MetricFamily, name string, labelName string, labelValue string) float64 {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == labelName && label.GetValue() == labelValue {
					return valueOf(metric)
				}
			}
		}
	}
	t.Fatalf("metric %s with %s=%s not found", name, labelName, labelValue)
	return 0
}

func valueOf(metric *dto.Metric) float64 {
	if metric.GetGauge() != nil {
		return metric.GetGauge().GetValue()
	}
	if metric.GetCounter() != nil {
		return metric.GetCounter().GetValue()
	}
	return 0
}

func boolPtr(value bool) *bool {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
