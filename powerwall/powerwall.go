// Package powerwall extracts information from a powerwall on demand.
package powerwall

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type HTTPMethod string

const (
	kGet  HTTPMethod = "GET"
	kPost HTTPMethod = "POST"
)

// Options describes the information needed to extract information
// from the Tesla Energy Gateway about your powerwalls.
type Options struct {
	// Gateway is the hostname or IP address of the Tesla
	// Energy gateway.
	Gateway string
	// Username should be the "customer" username for the gateway.
	// You'll have to setup these credentials by pointing your
	// browser at the gateway and going through the customer
	// account setup flow before using this monitor.
	Username string
	// Password should be the "customer" password for the gateway.
	Password string
}

// New returns a powerwall.Monitor that can extract information from
// the gateway.
func New(opts Options) (Monitor, error) {
	// Tesla Energy Gateway has an invalid SSL certificate.
	// We want to talk to it anyway.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	cli := &http.Client{
		Jar:       jar,
		Timeout:   5 * time.Second,
		Transport: tr,
	}
	r := &monitor{
		cli:     cli,
		opts:    opts,
		baseUrl: fmt.Sprintf("https://%s/api", opts.Gateway),
	}
	if err := r.login(); err != nil {
		return nil, err
	}
	return r, nil
}

type Monitor interface {
	io.Closer
	GetNetworks() ([]Network, error)
	GetSiteInfo() (*SiteInfo, error)
	GetOperation() (*Operation, error)
	GetConfig() (*Config, error)
	GetPowerwalls() (*Powerwalls, error)
	GetStatus() (*Status, error)
	GetSiteMaster() (*SiteMaster, error)
	GetAggregates() (*Aggregates, error)
	GetSOE() (*SOE, error)
	GetGridStatus() (*GridStatus, error)
	GetSolars() ([]Solar, error)
	GetInstaller() (*Installer, error)
}

type monitor struct {
	baseUrl   string
	cli       *http.Client
	opts      Options
	authToken string
}

const kCustomer = "customer"

type loginRequest struct {
	Username   string `json:"username"` // "customer"
	Email      string `json:"email"`
	Password   string `json:"password"`
	ForceSmOff bool   `json:"force_sm_off"`
}

type loginResponse struct {
	Email     string   `json:"email"`
	FirstName string   `json:"firstname"` // Tesla
	LastName  string   `json:"lastname"`  // Energy
	Roles     []string `json:"roles"`     // ["Home_Owner"]
	Token     string   `json:"token"`
	Provider  string   `json:"provider"`  // "Basic"
	LoginTime string   `json:"loginTime"` // YYYY-MM-DDTHH:MM:SS.XXXXXXXXX-HH:MM
}

func (m *monitor) issueRequest(method HTTPMethod, endpoint string, payload interface{}, response interface{}) error {
	var body io.Reader
	if payload != nil {
		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(payload)
		if err != nil {
			return fmt.Errorf("json Encode: %v", err)
		}
		body = &buf
	}
	hreq, err := http.NewRequest(string(method), fmt.Sprintf("%s%s", m.baseUrl, endpoint), body)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %v", err)
	}
	hresp, err := m.cli.Do(hreq)
	if err != nil {
		return fmt.Errorf("c.cli.Do(): %v", err)
	}
	if got, want := hresp.StatusCode, 200; got != want {
		return fmt.Errorf("basic login: got status code %d, want %d", got, want)
	}
	defer func() {
		if err := hresp.Body.Close(); err != nil {
			glog.Errorf("hresp.Body.Close(): %v", err)
		}
	}()
	bodyBytes, err := ioutil.ReadAll(hresp.Body)
	if err != nil {
		return fmt.Errorf("reading body of response: %v", err)
	}
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(response); err != nil {
		return fmt.Errorf("json Decode server response at endpoint %s: %v\nResponse:\n%s", endpoint, err, string(bodyBytes))
	}
	return nil
}

func (m *monitor) login() error {
	req := loginRequest{
		Username: kCustomer,
		Email:    m.opts.Username,
		Password: m.opts.Password,
	}
	var resp loginResponse
	if err := m.issueRequest(kPost, "/login/Basic", &req, &resp); err != nil {
		return err
	}
	m.authToken = resp.Token
	return nil
}

type IP struct {
	IPAddress string `json:"ip"`
	Netmask   int    `json:"netmask"`
}

type NetworkInfo struct {
	Name            string           `json:"network_name"`
	Networks        []IP             `json:"networks"`
	Gateway         string           `json:"gateway"`
	Interface       NetworkInterface `json:"interface"`
	State           string           `json:"state"`
	StateReason     string           `json:"state_reason"`
	SignalStrength  int              `json:"signal_strength"`
	HardwareAddress string           `json:"hw_address"`
}

type Network struct {
	Name      string           `json:"network_name"`
	Interface NetworkInterface `json:"interface"`
	DHCP      bool             `json:"dhcp"`
	Enabled   bool             `json:"enabled"`
	ExtraIPs  []IP             `json:"extra_ips"`
	Active    bool             `json:"active"`
	Primary   bool             `json:"primary"`
	Info      NetworkInfo      `json:"iface_network_info"`
}

func (m *monitor) GetNetworks() ([]Network, error) {
	var resp []Network
	if err := m.issueRequest(kGet, "/networks", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

type GridCode struct {
	Code         string `json:"grid_code"` // "60Hz_240V_s_UL1741SA:2018_ISO-NE"
	Voltage      int    `json:"grid_voltage_setting"`
	Frequency    int    `json:"grid_freq_setting"`
	PhaseSetting string `json:"grid_phase_setting"` // "Split
	Country      string `json:"country"`
	State        string `json:"state"`
	Distributor  string `json:"distributor"` // *
	Utility      string `json:"utility"`     // Eversource Energy (NSTAR-Cambridge Electric Light)
	Retailer     string `json:"retailer"`    // *
	Region       string `json:"region"`      // UL1741SA-IOS-NE:2018
	// can't determine the schema for overrides, so I'm ignoring it
	// In my case, they reduced the frequency shift when the batteries are full to
	// prevent problems with the UPS, so I see:
	// "grid_code_overrides":[{"name":"soc_freq_droop_config_df_max","value":2.5}]
}

type SiteInfo struct {
	MaxSystemEnergykWh     int64    `json:"max_system_energy_kWh"`
	MaxSystemPowerkW       int64    `json:"max_system_power_kW"`
	SiteName               string   `json:"site_name"`
	TimeZone               TimeZone `json:"timezone"`
	MaxSiteMeterPowerkW    int64    `json:"max_site_meter_power_kW"`
	MinSiteMeterPowerkW    int64    `json:"min_site_meter_power_kW"`
	NominalSystemEnergykWh float64  `json:"nominal_system_energy_kWh"`
	NominalSystemPowerkW   float64  `json:"nominal_system_power_kW"`
	GridCode               GridCode `json:"grid_code"`
}

func (m *monitor) GetSiteInfo() (*SiteInfo, error) {
	var resp SiteInfo
	if err := m.issueRequest(kGet, "/site_info", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *monitor) Close() error {
	return nil
}

type Operation struct {
	RealMode                OperatingMode `json:"real_mode"` // "backup"
	BackupReservePercent    float64       `json:"backup_reserve_percent"`
	FreqShiftLoadShedSOE    float64       `json:"freq_shift_load_shed_soe"`
	FreqShiftLoadShedDeltaF float64       `json:"freq_shift_load_shed_delta_f"`
}

func (m *monitor) GetOperation() (*Operation, error) {
	var resp Operation
	if err := m.issueRequest(kGet, "/operation", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type Config struct {
	VIN string `json:"vin"`
}

func (m *monitor) GetConfig() (*Config, error) {
	var resp Config
	if err := m.issueRequest(kGet, "/config", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type Diagnostic struct {
	Name       string `json:"name"`     // "Commissioning"
	Category   string `json:"category"` // "InternalComms"
	Disruptive bool   `json:"disruptive"`
	// inputs is null, so I cannot determine the schema
	Checks []DiagnosticCheck `json:"checks"`
}

type DiagnosticCheck struct {
	Name      string `json:"name"`   // CAN connectivity
	Status    string `json:"status"` // fail
	StartTime Time   `json:"start_time"`
	EndTime   Time   `json:"end_time"`
	Message   string `json:"message"`
	// tuple with results and debug, empty tuples, cannot determine schema
}

type Powerwall struct {
	UnusedType                  string `json:"Type"`
	PackagePartNumber           string
	PackageSerialNumber         string
	Type                        string               `json:"type"`       // acpw
	GridState                   GridState            `json:"grid_state"` // "Grid_Uncompliant"
	GridReconnectionTimeSeconds FloatDurationSeconds `json:"grid_reconnection_time_seconds"`
	UnderPhaseDetection         bool                 `json:"under_phase_detection"`
	Updating                    bool                 `json:"updating"`
	CommissioningDiagnostic     Diagnostic           `json:"commissioning_diagnostic"`
	UpdateDiagnostic            Diagnostic           `json:"update_diagnostic"`
	// bc_type: null ??
}

type Powerwalls struct {
	Enumerating                bool        `json:"enumerating"`
	Updating                   bool        `json:"updating"`
	CheckingIfOffGrid          bool        `json:"checking_if_offgrid"`
	RunningPhaseDetection      bool        `json:"running_phase_detection"`
	PhaseDetectionLastError    string      `json:"phase_detection_last_error"`
	BubbleShedding             bool        `json:"bubble_shedding"`
	OnGridCheckError           string      `json:"on_grid_check_error"`
	GridQualifying             bool        `json:"grid_qualifying"`
	GridCodeValidating         bool        `json:"grid_code_validating"`
	PhaseDetectionNotAvailable bool        `json:"phase_detection_not_available"`
	Powerwalls                 []Powerwall `json:"powerwalls"`
}

func (m *monitor) GetPowerwalls() (*Powerwalls, error) {
	var rval Powerwalls
	if err := m.issueRequest(kGet, "/powerwalls", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type Status struct {
	StartTime       Time     `json:"start_time"`
	UpTime          Duration `json:"up_time_seconds"`
	IsNew           bool     `json:"is_new"`
	Version         string   `json:"version"`          // 20.49.0
	GitHash         string   `json:"git_hash"`         // hexadecimal sequence
	CommissionCount int      `json:"commission_count"` // 0
	// jrester code suggests gateway1 == "hec", gateway2 == "teg",
	// and a not yet attributed value "smc"
	DeviceType string `json:"device_type"` // "hec"
	// jrester code suggests sync type values can include
	// "v1", "v2", and "v2.1"
	SyncType string `json:"sync_type"` // v1
}

func (m *monitor) GetStatus() (*Status, error) {
	var rval Status
	if err := m.issueRequest(kGet, "/status", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type SiteMaster struct {
	Status           string `json:"status"` // StatusUp
	Running          bool   `json:"running"`
	ConnectedToTesla bool   `json:"connected_to_tesla"`
	PowerSupplyMode  bool   `json:"power_supply_mode"`
}

func (m *monitor) GetSiteMaster() (*SiteMaster, error) {
	var rval SiteMaster
	if err := m.issueRequest(kGet, "/sitemaster", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type MeterDetails struct {
	LastCommunicationTime             Time    `json:"last_communication_time"` // YYYY-MM-DDTHH:MM:SS-HH:MM
	InstantPower                      float64 `json:"instant_power"`
	InstantReactivePower              float64 `json:"instant_reactive_power"`
	InstantApparentPower              float64 `json:"instant_apparant_power"`
	Frequency                         float64 `json:"frequency"`
	EnergyExported                    float64 `json:"energy_exported"`
	EnergyImported                    float64 `json:"energy_imported"`
	InstantAverageVoltage             float64 `json:"instant_average_voltage"`
	InstantTotalCurrent               float64 `json:"instant_total_current"`
	IACurrent                         float64 `json:"i_a_current"`
	IBCurrent                         float64 `json:"i_b_current"`
	ICCurrent                         float64 `json:"i_c_current"`
	LastPhaseVoltageCommunicationTime Time    `json:"last_phase_voltage_communication_time"`
	LastPhasePowerCommunicationTime   Time    `json:"last_phase_power_communication_time"`
	// Would like to turn Timeout into a time.Duration, but I need to know units.
	Timeout int64 `json:"timeout"`
}

type Aggregates struct {
	Site    MeterDetails `json:"site"`
	Battery MeterDetails `json:"battery"`
	Load    MeterDetails `json:"load"`
	Solar   MeterDetails `json:"solar"`
}

func (m *monitor) GetAggregates() (*Aggregates, error) {
	var rval Aggregates
	if err := m.issueRequest(kGet, "/meters/aggregates", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type SOE struct {
	Percentage float64 `json:"percentage"`
}

func (m *monitor) GetSOE() (*SOE, error) {
	var rval SOE
	if err := m.issueRequest(kGet, "/system_status/soe", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type GridStatus struct {
	Status SystemStatus `json:"grid_status"`          // SystemGridConnected
	Active bool         `json:"grid_services_active"` // false in normal operation.  Unclear what this means.
}

func (m *monitor) GetGridStatus() (*GridStatus, error) {
	var rval GridStatus
	if err := m.issueRequest(kGet, "/system_status/grid_status", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

type Solar struct {
	Brand            string `json:"brand"`              // "SolarEdge Technologies"
	Model            string `json:"model"`              // SE 1000A-US (240V)
	PowerRatingWatts int    `json:"power_rating_watts"` // 15170
}

func (m *monitor) GetSolars() ([]Solar, error) {
	var rval []Solar
	if err := m.issueRequest(kGet, "/solars", nil, &rval); err != nil {
		return nil, err
	}
	return rval, nil
}

type Installer struct {
	Company                string   `json:"company"`
	CustomerID             string   `json:"customer_id"`
	Phone                  string   `json:"phone"`
	Email                  string   `json:"email"`
	Location               string   `json:"location"`
	Mounting               string   `json:"mounting"`
	Wiring                 string   `json:"wiring"`
	BackupConfiguration    string   `json:"backup_configuration"`
	SolarInstallation      string   `json:"solar_installation"`
	HasStackKit            bool     `json:"has_stack_kit"`
	HasPowerlineToEthernet bool     `json:"has_powerline_to_ethernet"`
	RunSitemaster          bool     `json:"run_sitemaster"`
	VerifiedConfig         bool     `json:"verified_config"`
	InstallationTypes      []string `json:"installation_types"`
}

func (m *monitor) GetInstaller() (*Installer, error) {
	var rval Installer
	if err := m.issueRequest(kGet, "/installer", nil, &rval); err != nil {
		return nil, err
	}
	return &rval, nil
}

// getlogs returns a gzipped tarball of logs
