package m3

type Target struct {
	Name               string
	BaseURL            string
	Username           string
	Password           string
	APIVersion         string
	InsecureSkipVerify bool
}

type Snapshot struct {
	TargetName               string
	BaseURL                  string
	PowerDistributions       []PowerDistribution
	PowerServiceStatus       Status
	EnvironmentServiceStatus Status
	AlarmCounts              AlarmCounts
	MostCriticalAlarm        Alarm
	Suppliers                []Supplier
	Temperatures             []EnvironmentSensor
	Humidities               []EnvironmentSensor
	EnvironmentInputs        []EnvironmentInput
}

type PowerDistribution struct {
	ID             string
	Identification Identification
	Status         Status
	PowerBank      PowerBankStatus
	Inputs         []PowerResource
	Outputs        []PowerResource
	Outlets        []PowerResource
}

type Supplier struct {
	ID             string
	Identification Identification
	Measures       Measures
	Summary        SupplierSummary
}

type PowerResource struct {
	ID             string
	Kind           string
	Identification Identification
	Status         Status
	Measures       Measures
}

type Identification struct {
	UUID            string `json:"uuid"`
	PhysicalName    string `json:"physicalName"`
	FriendlyName    string `json:"friendlyName"`
	SerialNumber    string `json:"serialNumber"`
	PartNumber      string `json:"partNumber"`
	ReferenceNumber string `json:"referenceNumber"`
	Manufacturer    string `json:"manufacturer"`
	Vendor          string `json:"vendor"`
	Model           string `json:"model"`
	Type            string `json:"type"`
	ProductName     string `json:"productName"`
	FirmwareVersion string `json:"firmwareVersion"`
	Name            string `json:"name"`
}

func (i Identification) DisplayName() string {
	if i.FriendlyName != "" {
		return i.FriendlyName
	}
	if i.Name != "" {
		return i.Name
	}
	if i.PhysicalName != "" {
		return i.PhysicalName
	}
	return i.UUID
}

type Status struct {
	Operating             string   `json:"operating"`
	Health                string   `json:"health"`
	Mode                  string   `json:"mode"`
	SupplierPowerQuality  string   `json:"supplierPowerQuality"`
	BootloaderMode        *bool    `json:"bootloaderMode"`
	CommunicationFault    *bool    `json:"communicationFault"`
	ConfigurationFault    *bool    `json:"configurationFault"`
	EmergencySwitchOff    *bool    `json:"emergencySwitchOff"`
	FanFault              *bool    `json:"fanFault"`
	FrequencyOutOfRange   *bool    `json:"frequencyOutOfRange"`
	InRange               *bool    `json:"inRange"`
	InternalFailure       *bool    `json:"internalFailure"`
	Overload              *bool    `json:"overload"`
	ShortCircuit          *bool    `json:"shortCircuit"`
	ShutdownImminent      *bool    `json:"shutdownImminent"`
	Supplied              *bool    `json:"supplied"`
	Supply                *bool    `json:"supply"`
	SwitchedOn            *bool    `json:"switchedOn"`
	SystemAlarm           *bool    `json:"systemAlarm"`
	TemperatureOutOfRange *bool    `json:"temperatureOutOfRange"`
	VoltageOutOfRange     *bool    `json:"voltageOutOfRange"`
	VoltageTooHigh        *bool    `json:"voltageTooHigh"`
	VoltageTooLow         *bool    `json:"voltageTooLow"`
	DelayBeforeSwitchOff  *float64 `json:"delayBeforeSwitchOff"`
	DelayBeforeSwitchOn   *float64 `json:"delayBeforeSwitchOn"`
}

func (s Status) BoolMetrics() map[string]bool {
	return presentBools(map[string]*bool{
		"bootloader_mode":          s.BootloaderMode,
		"communication_fault":      s.CommunicationFault,
		"configuration_fault":      s.ConfigurationFault,
		"emergency_switch_off":     s.EmergencySwitchOff,
		"fan_fault":                s.FanFault,
		"frequency_out_of_range":   s.FrequencyOutOfRange,
		"in_range":                 s.InRange,
		"internal_failure":         s.InternalFailure,
		"overload":                 s.Overload,
		"short_circuit":            s.ShortCircuit,
		"shutdown_imminent":        s.ShutdownImminent,
		"supplied":                 s.Supplied,
		"supply":                   s.Supply,
		"switched_on":              s.SwitchedOn,
		"system_alarm":             s.SystemAlarm,
		"temperature_out_of_range": s.TemperatureOutOfRange,
		"voltage_out_of_range":     s.VoltageOutOfRange,
		"voltage_too_high":         s.VoltageTooHigh,
		"voltage_too_low":          s.VoltageTooLow,
	})
}

type Measures struct {
	ActivePower     *float64 `json:"activePower"`
	ApparentPower   *float64 `json:"apparentPower"`
	AverageEnergy   *float64 `json:"averageEnergy"`
	CumulatedEnergy *float64 `json:"cumulatedEnergy"`
	Current         *float64 `json:"current"`
	Efficiency      *float64 `json:"efficiency"`
	Frequency       *float64 `json:"frequency"`
	PercentLoad     *float64 `json:"percentLoad"`
	PowerFactor     *float64 `json:"powerFactor"`
	Voltage         *float64 `json:"voltage"`
}

func (m Measures) Values() map[string]float64 {
	return presentFloats(map[string]*float64{
		"activePower":     m.ActivePower,
		"apparentPower":   m.ApparentPower,
		"averageEnergy":   m.AverageEnergy,
		"cumulatedEnergy": m.CumulatedEnergy,
		"current":         m.Current,
		"efficiency":      m.Efficiency,
		"frequency":       m.Frequency,
		"percentLoad":     m.PercentLoad,
		"powerFactor":     m.PowerFactor,
		"voltage":         m.Voltage,
	})
}

type SupplierSummary struct {
	CapacityRuntime            float64 `json:"capacityRuntime"`
	LoadPercent                float64 `json:"loadPercent"`
	LowCapacityAlarm           float64 `json:"lowCapacityAlarm"`
	Mode                       string  `json:"mode"`
	Quality                    string  `json:"quality"`
	PoweringFor                float64 `json:"poweringFor"`
	ProtectingFor              float64 `json:"protectingFor"`
	ProtectionCapacityPercent  float64 `json:"protectionCapacityPercent"`
	ProtectionCapacityRuntime  float64 `json:"protectionCapacityRuntime"`
	ProtectionLowCapacityAlarm bool    `json:"protectionLowCapacityAlarm"`
}

type PowerBankStatus struct {
	Operating                string `json:"operating"`
	Health                   string `json:"health"`
	LastTestResult           string `json:"lastTestResult"`
	StoragePresent           string `json:"storagePresent"`
	TestStatus               string `json:"testStatus"`
	CriticalLowStateOfCharge *bool  `json:"criticalLowStateOfCharge"`
	InternalFailure          *bool  `json:"internalFailure"`
	LCMExpired               *bool  `json:"lcmExpired"`
	LowStateOfCharge         *bool  `json:"lowStateOfCharge"`
	Supplied                 *bool  `json:"supplied"`
	Supply                   *bool  `json:"supply"`
	TestFailed               *bool  `json:"testFailed"`
}

func (p PowerBankStatus) BoolMetrics() map[string]bool {
	return presentBools(map[string]*bool{
		"critical_low_state_of_charge": p.CriticalLowStateOfCharge,
		"internal_failure":             p.InternalFailure,
		"lcm_expired":                  p.LCMExpired,
		"low_state_of_charge":          p.LowStateOfCharge,
		"supplied":                     p.Supplied,
		"supply":                       p.Supply,
		"test_failed":                  p.TestFailed,
	})
}

func presentBools(values map[string]*bool) map[string]bool {
	out := make(map[string]bool, len(values))
	for name, value := range values {
		if value != nil {
			out[name] = *value
		}
	}
	return out
}

func presentFloats(values map[string]*float64) map[string]float64 {
	out := make(map[string]float64, len(values))
	for name, value := range values {
		if value != nil {
			out[name] = *value
		}
	}
	return out
}

type AlarmCounts struct {
	ActiveCount float64 `json:"activeCount"`
	TotalCount  float64 `json:"totalCount"`
}

type Alarm struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Code        string `json:"code"`
	Level       string `json:"level"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
	Device      struct {
		Name string `json:"name"`
	} `json:"device"`
}

type EnvironmentSensor struct {
	ID         string  `json:"id"`
	UUID       string  `json:"uuid"`
	Name       string  `json:"name"`
	Position   string  `json:"position"`
	Elevation  string  `json:"elevation"`
	Measure    float64 `json:"measure"`
	AlarmLevel string  `json:"alarmLevel"`
}

type EnvironmentInput struct {
	ID         string `json:"id"`
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Position   string `json:"position"`
	Elevation  string `json:"elevation"`
	Active     bool   `json:"active"`
	AlarmLevel string `json:"alarmLevel"`
}

type memberList struct {
	Count   int `json:"members@count"`
	Members []struct {
		ID string `json:"@id"`
	} `json:"members"`
}

type tokenResponse struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
}
