package exporter

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.synost.net/synost/eaton-m3-exporter/internal/m3"
)

type Client interface {
	Target() m3.Target
	FetchSnapshot(ctx context.Context) (m3.Snapshot, error)
}

type Collector struct {
	clients []Client
	timeout time.Duration
	logger  *slog.Logger
	errors  map[string]float64
	mu      sync.Mutex

	up                         *prometheus.Desc
	scrapeDuration             *prometheus.Desc
	scrapeErrors               *prometheus.Desc
	powerDistributionInfo      *prometheus.Desc
	powerResourceInfo          *prometheus.Desc
	powerDistributionStatus    *prometheus.Desc
	powerServiceStatus         *prometheus.Desc
	environmentStatus          *prometheus.Desc
	statusBool                 *prometheus.Desc
	statusDelay                *prometheus.Desc
	alarmActiveCount           *prometheus.Desc
	alarmTotalCount            *prometheus.Desc
	alarmMostCritical          *prometheus.Desc
	measureDescs               map[string]*prometheus.Desc
	supplierRuntimeDescs       map[string]*prometheus.Desc
	supplierLowAlarm           *prometheus.Desc
	supplierProtectionLowAlarm *prometheus.Desc
	supplierMode               *prometheus.Desc
	supplierQuality            *prometheus.Desc
	powerBankStatus            *prometheus.Desc
	powerBankBool              *prometheus.Desc
	temperatureKelvin          *prometheus.Desc
	humidityPercent            *prometheus.Desc
	environmentInputActive     *prometheus.Desc
	environmentAlarmLevel      *prometheus.Desc
}

func NewCollector(clients []Client, timeout time.Duration, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Collector{
		clients: clients,
		timeout: timeout,
		logger:  logger,
		errors:  make(map[string]float64),
		up: prometheus.NewDesc(
			"eaton_m3_up",
			"Whether the last scrape of the Eaton Network M3 target succeeded.",
			[]string{"target", "base_url"}, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"eaton_m3_scrape_duration_seconds",
			"Duration of the Eaton Network M3 target scrape.",
			[]string{"target", "base_url"}, nil,
		),
		scrapeErrors: prometheus.NewDesc(
			"eaton_m3_scrape_errors_total",
			"Total number of Eaton Network M3 target scrape errors observed by this exporter process.",
			[]string{"target", "base_url"}, nil,
		),
		powerDistributionInfo: prometheus.NewDesc(
			"eaton_m3_power_distribution_info",
			"Eaton Network M3 power distribution identity information.",
			[]string{"target", "base_url", "power_distribution", "uuid", "name", "serial_number", "part_number", "model", "firmware_version"}, nil,
		),
		powerResourceInfo: prometheus.NewDesc(
			"eaton_m3_power_resource_info",
			"Eaton Network M3 input, output, or outlet identity information.",
			[]string{"target", "base_url", "resource_type", "resource_id", "uuid", "name"}, nil,
		),
		powerDistributionStatus: prometheus.NewDesc(
			"eaton_m3_power_distribution_status",
			"Current Eaton Network M3 power distribution enum status. Value is 1 for the current state.",
			[]string{"target", "base_url", "power_distribution", "state_type", "state"}, nil,
		),
		powerServiceStatus: prometheus.NewDesc(
			"eaton_m3_power_service_status",
			"Current Eaton Network M3 power service enum status. Value is 1 for the current state.",
			[]string{"target", "base_url", "state_type", "state"}, nil,
		),
		environmentStatus: prometheus.NewDesc(
			"eaton_m3_environment_status",
			"Current Eaton Network M3 environment service enum status. Value is 1 for the current state.",
			[]string{"target", "base_url", "state_type", "state"}, nil,
		),
		statusBool: prometheus.NewDesc(
			"eaton_m3_status_bool",
			"Boolean status fields exposed by the Eaton Network M3 API.",
			[]string{"target", "base_url", "resource_type", "resource_id", "name", "field"}, nil,
		),
		statusDelay: prometheus.NewDesc(
			"eaton_m3_status_delay_seconds",
			"Delay fields exposed by the Eaton Network M3 API. Negative values are passed through from the device.",
			[]string{"target", "base_url", "resource_type", "resource_id", "name", "field"}, nil,
		),
		alarmActiveCount: prometheus.NewDesc(
			"eaton_m3_alarm_active_count",
			"Active alarm count reported by the Eaton Network M3 card.",
			[]string{"target", "base_url"}, nil,
		),
		alarmTotalCount: prometheus.NewDesc(
			"eaton_m3_alarm_total_count",
			"Total alarm count reported by the Eaton Network M3 card.",
			[]string{"target", "base_url"}, nil,
		),
		alarmMostCritical: prometheus.NewDesc(
			"eaton_m3_alarm_most_critical",
			"Most critical alarm reported by the Eaton Network M3 card. Value is 1 when an alarm is present.",
			[]string{"target", "base_url", "id", "state", "code", "level", "description", "device"}, nil,
		),
		measureDescs: map[string]*prometheus.Desc{
			"activePower":     newMeasureDesc("eaton_m3_active_power_watts", "Active power reported by Eaton Network M3."),
			"apparentPower":   newMeasureDesc("eaton_m3_apparent_power_va", "Apparent power reported by Eaton Network M3."),
			"averageEnergy":   newMeasureDesc("eaton_m3_average_energy_watt_hours", "Average energy reported by Eaton Network M3."),
			"cumulatedEnergy": newMeasureDesc("eaton_m3_cumulated_energy_watt_hours", "Cumulated energy reported by Eaton Network M3."),
			"current":         newMeasureDesc("eaton_m3_current_amperes", "Current reported by Eaton Network M3."),
			"efficiency":      newMeasureDesc("eaton_m3_efficiency_percent", "Efficiency reported by Eaton Network M3."),
			"frequency":       newMeasureDesc("eaton_m3_frequency_hertz", "Frequency reported by Eaton Network M3."),
			"percentLoad":     newMeasureDesc("eaton_m3_load_percent", "Load percentage reported by Eaton Network M3."),
			"powerFactor":     newMeasureDesc("eaton_m3_power_factor", "Power factor reported by Eaton Network M3."),
			"voltage":         newMeasureDesc("eaton_m3_voltage_volts", "Voltage reported by Eaton Network M3."),
		},
		supplierRuntimeDescs: map[string]*prometheus.Desc{
			"capacity_runtime":            newSupplierDesc("eaton_m3_capacity_runtime_seconds", "Capacity runtime reported by Eaton Network M3."),
			"load_percent":                newSupplierDesc("eaton_m3_supplier_load_percent", "Supplier load percentage reported by Eaton Network M3."),
			"powering_for":                newSupplierDesc("eaton_m3_powering_for_seconds", "Powering duration reported by Eaton Network M3."),
			"protecting_for":              newSupplierDesc("eaton_m3_protecting_for_seconds", "Protecting duration reported by Eaton Network M3."),
			"protection_capacity_percent": newSupplierDesc("eaton_m3_protection_capacity_percent", "Protection capacity percentage reported by Eaton Network M3."),
			"protection_capacity_runtime": newSupplierDesc("eaton_m3_protection_capacity_runtime_seconds", "Protection capacity runtime reported by Eaton Network M3."),
		},
		supplierLowAlarm: prometheus.NewDesc(
			"eaton_m3_supplier_low_capacity_alarm",
			"Low capacity alarm state reported by Eaton Network M3 supplier summary.",
			[]string{"target", "base_url", "supplier"}, nil,
		),
		supplierProtectionLowAlarm: prometheus.NewDesc(
			"eaton_m3_supplier_protection_low_capacity_alarm",
			"Protection low capacity alarm state reported by Eaton Network M3 supplier summary.",
			[]string{"target", "base_url", "supplier"}, nil,
		),
		supplierMode: prometheus.NewDesc(
			"eaton_m3_supplier_mode",
			"Supplier mode enum reported by Eaton Network M3. Value is 1 for the current mode.",
			[]string{"target", "base_url", "supplier", "mode"}, nil,
		),
		supplierQuality: prometheus.NewDesc(
			"eaton_m3_supplier_quality",
			"Supplier quality enum reported by Eaton Network M3. Value is 1 for the current quality.",
			[]string{"target", "base_url", "supplier", "quality"}, nil,
		),
		powerBankStatus: prometheus.NewDesc(
			"eaton_m3_power_bank_status",
			"Power bank enum status reported by Eaton Network M3. Value is 1 for the current state.",
			[]string{"target", "base_url", "power_distribution", "state_type", "state"}, nil,
		),
		powerBankBool: prometheus.NewDesc(
			"eaton_m3_power_bank_bool",
			"Boolean power bank status fields reported by Eaton Network M3.",
			[]string{"target", "base_url", "power_distribution", "field"}, nil,
		),
		temperatureKelvin: prometheus.NewDesc(
			"eaton_m3_temperature_kelvin",
			"Environment temperature returned by the Eaton Network M3 API in Kelvin.",
			[]string{"target", "base_url", "sensor", "name", "position", "elevation"}, nil,
		),
		humidityPercent: prometheus.NewDesc(
			"eaton_m3_humidity_percent",
			"Environment humidity percentage returned by the Eaton Network M3 API.",
			[]string{"target", "base_url", "sensor", "name", "position", "elevation"}, nil,
		),
		environmentInputActive: prometheus.NewDesc(
			"eaton_m3_environment_input_active",
			"Environment digital input active state returned by the Eaton Network M3 API.",
			[]string{"target", "base_url", "input", "name", "position", "elevation"}, nil,
		),
		environmentAlarmLevel: prometheus.NewDesc(
			"eaton_m3_environment_alarm_level",
			"Environment alarm level enum reported by Eaton Network M3. Value is 1 for the current level.",
			[]string{"target", "base_url", "resource_type", "resource_id", "name", "level"}, nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.scrapeErrors
	ch <- c.powerDistributionInfo
	ch <- c.powerResourceInfo
	ch <- c.powerDistributionStatus
	ch <- c.powerServiceStatus
	ch <- c.environmentStatus
	ch <- c.statusBool
	ch <- c.statusDelay
	ch <- c.alarmActiveCount
	ch <- c.alarmTotalCount
	ch <- c.alarmMostCritical
	for _, desc := range c.measureDescs {
		ch <- desc
	}
	for _, desc := range c.supplierRuntimeDescs {
		ch <- desc
	}
	ch <- c.supplierLowAlarm
	ch <- c.supplierProtectionLowAlarm
	ch <- c.supplierMode
	ch <- c.supplierQuality
	ch <- c.powerBankStatus
	ch <- c.powerBankBool
	ch <- c.temperatureKelvin
	ch <- c.humidityPercent
	ch <- c.environmentInputActive
	ch <- c.environmentAlarmLevel
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup
	for _, client := range c.clients {
		client := client
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.collectTarget(client, ch)
		}()
	}
	wg.Wait()
}

func (c *Collector) collectTarget(client Client, ch chan<- prometheus.Metric) {
	target := client.Target()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	snapshot, err := client.FetchSnapshot(ctx)
	duration := time.Since(start).Seconds()
	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration, target.Name, target.BaseURL)
	if err != nil {
		c.logger.Warn("target scrape failed", "target", target.Name, "error", err)
		c.incrementError(target.Name)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, target.Name, target.BaseURL)
		ch <- prometheus.MustNewConstMetric(c.scrapeErrors, prometheus.CounterValue, c.errorCount(target.Name), target.Name, target.BaseURL)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, target.Name, target.BaseURL)
	ch <- prometheus.MustNewConstMetric(c.scrapeErrors, prometheus.CounterValue, c.errorCount(target.Name), target.Name, target.BaseURL)
	c.emitSnapshot(ch, snapshot)
}

func (c *Collector) emitSnapshot(ch chan<- prometheus.Metric, s m3.Snapshot) {
	labels := []string{s.TargetName, s.BaseURL}
	ch <- prometheus.MustNewConstMetric(c.powerServiceStatus, prometheus.GaugeValue, 1, append(labels, "operating", s.PowerServiceStatus.Operating)...)
	ch <- prometheus.MustNewConstMetric(c.powerServiceStatus, prometheus.GaugeValue, 1, append(labels, "health", s.PowerServiceStatus.Health)...)
	ch <- prometheus.MustNewConstMetric(c.environmentStatus, prometheus.GaugeValue, 1, append(labels, "operating", s.EnvironmentServiceStatus.Operating)...)
	ch <- prometheus.MustNewConstMetric(c.environmentStatus, prometheus.GaugeValue, 1, append(labels, "health", s.EnvironmentServiceStatus.Health)...)

	ch <- prometheus.MustNewConstMetric(c.alarmActiveCount, prometheus.GaugeValue, s.AlarmCounts.ActiveCount, labels...)
	ch <- prometheus.MustNewConstMetric(c.alarmTotalCount, prometheus.GaugeValue, s.AlarmCounts.TotalCount, labels...)
	if s.MostCriticalAlarm.ID != "" {
		ch <- prometheus.MustNewConstMetric(c.alarmMostCritical, prometheus.GaugeValue, 1, append(labels, s.MostCriticalAlarm.ID, s.MostCriticalAlarm.State, s.MostCriticalAlarm.Code, s.MostCriticalAlarm.Level, s.MostCriticalAlarm.Description, s.MostCriticalAlarm.Device.Name)...)
	}

	for _, pd := range s.PowerDistributions {
		c.emitPowerDistribution(ch, labels, pd)
	}
	for _, supplier := range s.Suppliers {
		c.emitSupplier(ch, labels, supplier)
	}
	for _, sensor := range s.Temperatures {
		ch <- prometheus.MustNewConstMetric(c.temperatureKelvin, prometheus.GaugeValue, sensor.Measure, append(labels, sensor.ID, sensor.Name, sensor.Position, sensor.Elevation)...)
		ch <- prometheus.MustNewConstMetric(c.environmentAlarmLevel, prometheus.GaugeValue, 1, append(labels, "temperature", sensor.ID, sensor.Name, sensor.AlarmLevel)...)
	}
	for _, sensor := range s.Humidities {
		ch <- prometheus.MustNewConstMetric(c.humidityPercent, prometheus.GaugeValue, sensor.Measure, append(labels, sensor.ID, sensor.Name, sensor.Position, sensor.Elevation)...)
		ch <- prometheus.MustNewConstMetric(c.environmentAlarmLevel, prometheus.GaugeValue, 1, append(labels, "humidity", sensor.ID, sensor.Name, sensor.AlarmLevel)...)
	}
	for _, input := range s.EnvironmentInputs {
		ch <- prometheus.MustNewConstMetric(c.environmentInputActive, prometheus.GaugeValue, boolFloat(input.Active), append(labels, input.ID, input.Name, input.Position, input.Elevation)...)
		ch <- prometheus.MustNewConstMetric(c.environmentAlarmLevel, prometheus.GaugeValue, 1, append(labels, "environment_input", input.ID, input.Name, input.AlarmLevel)...)
	}
}

func (c *Collector) emitPowerDistribution(ch chan<- prometheus.Metric, labels []string, pd m3.PowerDistribution) {
	id := pd.ID
	name := pd.Identification.DisplayName()
	ch <- prometheus.MustNewConstMetric(c.powerDistributionInfo, prometheus.GaugeValue, 1, append(labels, id, pd.Identification.UUID, name, pd.Identification.SerialNumber, pd.Identification.PartNumber, pd.Identification.Model, pd.Identification.FirmwareVersion)...)
	c.emitStatus(ch, c.powerDistributionStatus, append(labels, id), pd.Status)
	c.emitStatusBools(ch, labels, "power_distribution", id, name, pd.Status)
	c.emitPowerBank(ch, labels, id, pd.PowerBank)
	for _, resource := range pd.Inputs {
		c.emitPowerResource(ch, labels, resource)
	}
	for _, resource := range pd.Outputs {
		c.emitPowerResource(ch, labels, resource)
	}
	for _, resource := range pd.Outlets {
		c.emitPowerResource(ch, labels, resource)
	}
}

func (c *Collector) emitPowerResource(ch chan<- prometheus.Metric, labels []string, resource m3.PowerResource) {
	name := resource.Identification.DisplayName()
	ch <- prometheus.MustNewConstMetric(c.powerResourceInfo, prometheus.GaugeValue, 1, append(labels, resource.Kind, resource.ID, resource.Identification.UUID, name)...)
	c.emitStatusBools(ch, labels, resource.Kind, resource.ID, name, resource.Status)
	if resource.Status.DelayBeforeSwitchOff != nil {
		ch <- prometheus.MustNewConstMetric(c.statusDelay, prometheus.GaugeValue, *resource.Status.DelayBeforeSwitchOff, append(labels, resource.Kind, resource.ID, name, "delay_before_switch_off")...)
	}
	if resource.Status.DelayBeforeSwitchOn != nil {
		ch <- prometheus.MustNewConstMetric(c.statusDelay, prometheus.GaugeValue, *resource.Status.DelayBeforeSwitchOn, append(labels, resource.Kind, resource.ID, name, "delay_before_switch_on")...)
	}
	for field, value := range resource.Measures.Values() {
		if desc, ok := c.measureDescs[field]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, append(labels, resource.Kind, resource.ID, name)...)
		}
	}
}

func (c *Collector) emitSupplier(ch chan<- prometheus.Metric, labels []string, supplier m3.Supplier) {
	name := supplier.Identification.DisplayName()
	if name == "" {
		name = supplier.ID
	}
	for field, value := range supplier.Measures.Values() {
		if desc, ok := c.measureDescs[field]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, append(labels, "supplier", supplier.ID, name)...)
		}
	}
	summary := supplier.Summary
	values := map[string]float64{
		"capacity_runtime":            summary.CapacityRuntime,
		"load_percent":                summary.LoadPercent,
		"powering_for":                summary.PoweringFor,
		"protecting_for":              summary.ProtectingFor,
		"protection_capacity_percent": summary.ProtectionCapacityPercent,
		"protection_capacity_runtime": summary.ProtectionCapacityRuntime,
	}
	for field, value := range values {
		if desc, ok := c.supplierRuntimeDescs[field]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, append(labels, supplier.ID)...)
		}
	}
	ch <- prometheus.MustNewConstMetric(c.supplierLowAlarm, prometheus.GaugeValue, summary.LowCapacityAlarm, append(labels, supplier.ID)...)
	ch <- prometheus.MustNewConstMetric(c.supplierProtectionLowAlarm, prometheus.GaugeValue, boolFloat(summary.ProtectionLowCapacityAlarm), append(labels, supplier.ID)...)
	if summary.Mode != "" {
		ch <- prometheus.MustNewConstMetric(c.supplierMode, prometheus.GaugeValue, 1, append(labels, supplier.ID, summary.Mode)...)
	}
	if summary.Quality != "" {
		ch <- prometheus.MustNewConstMetric(c.supplierQuality, prometheus.GaugeValue, 1, append(labels, supplier.ID, summary.Quality)...)
	}
}

func (c *Collector) emitStatus(ch chan<- prometheus.Metric, desc *prometheus.Desc, labels []string, status m3.Status) {
	if status.Operating != "" {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, append(labels, "operating", status.Operating)...)
	}
	if status.Health != "" {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, append(labels, "health", status.Health)...)
	}
	if status.Mode != "" {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, append(labels, "mode", status.Mode)...)
	}
	if status.SupplierPowerQuality != "" {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, append(labels, "supplier_power_quality", status.SupplierPowerQuality)...)
	}
}

func (c *Collector) emitStatusBools(ch chan<- prometheus.Metric, labels []string, resourceType string, resourceID string, name string, status m3.Status) {
	for field, value := range status.BoolMetrics() {
		ch <- prometheus.MustNewConstMetric(c.statusBool, prometheus.GaugeValue, boolFloat(value), append(labels, resourceType, resourceID, name, field)...)
	}
}

func (c *Collector) emitPowerBank(ch chan<- prometheus.Metric, labels []string, pdID string, status m3.PowerBankStatus) {
	if status.Operating != "" {
		ch <- prometheus.MustNewConstMetric(c.powerBankStatus, prometheus.GaugeValue, 1, append(labels, pdID, "operating", status.Operating)...)
	}
	if status.Health != "" {
		ch <- prometheus.MustNewConstMetric(c.powerBankStatus, prometheus.GaugeValue, 1, append(labels, pdID, "health", status.Health)...)
	}
	if status.LastTestResult != "" {
		ch <- prometheus.MustNewConstMetric(c.powerBankStatus, prometheus.GaugeValue, 1, append(labels, pdID, "last_test_result", status.LastTestResult)...)
	}
	if status.StoragePresent != "" {
		ch <- prometheus.MustNewConstMetric(c.powerBankStatus, prometheus.GaugeValue, 1, append(labels, pdID, "storage_present", status.StoragePresent)...)
	}
	if status.TestStatus != "" {
		ch <- prometheus.MustNewConstMetric(c.powerBankStatus, prometheus.GaugeValue, 1, append(labels, pdID, "test_status", status.TestStatus)...)
	}
	for field, value := range status.BoolMetrics() {
		ch <- prometheus.MustNewConstMetric(c.powerBankBool, prometheus.GaugeValue, boolFloat(value), append(labels, pdID, field)...)
	}
}

func (c *Collector) incrementError(target string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errors[target]++
}

func (c *Collector) errorCount(target string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errors[target]
}

func newMeasureDesc(name string, help string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, []string{"target", "base_url", "resource_type", "resource_id", "name"}, nil)
}

func newSupplierDesc(name string, help string) *prometheus.Desc {
	return prometheus.NewDesc(name, help, []string{"target", "base_url", "supplier"}, nil)
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
