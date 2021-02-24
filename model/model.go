package model

import (
	"fmt"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"regexp"
	"strconv"
	"time"
)

// FixedInfo is unlikely to change from poll to poll,
// so we assume these fields have fixed values.
type FixedInfo struct {
	// from site info:
	NominalSystemEnergykWh float64
	NominalSystemPowerkW   float64
	SiteName               string
	// from powerwalls:
	NumPowerwalls          int
	PowerwallSerialNumbers []string
	// from config:
	VIN string
	// from solars:
	TotalSolarPowerRatingWatts int
	// nothing usefin in installer.
}

func fetchFixedInfo(mon powerwall.Monitor) (*FixedInfo, error) {
	si, err := mon.GetSiteInfo()
	if err != nil {
		return nil, fmt.Errorf("mon.GetSiteInfo(): %v", err)
	}
	pws, err := mon.GetPowerwalls()
	if err != nil {
		return nil, fmt.Errorf("mon.GetPowerwalls(): %v", err)
	}
	config, err := mon.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("mon.GetConfig(): %v", err)
	}
	solars, err := mon.GetSolars()
	if err != nil {
		return nil, fmt.Errorf("mon.GetSolars(): %v", err)
	}
	fi := FixedInfo{
		NominalSystemEnergykWh: si.NominalSystemEnergykWh,
		NominalSystemPowerkW:   si.NominalSystemPowerkW,
		SiteName:               si.SiteName,
		NumPowerwalls:          len(pws.Powerwalls),
		PowerwallSerialNumbers: func() []string {
			var rval []string
			for _, pw := range pws.Powerwalls {
				rval = append(rval, pw.PackageSerialNumber)
			}
			return rval
		}(),
		VIN: config.VIN,
		TotalSolarPowerRatingWatts: func() int {
			var rval int
			for _, s := range solars {
				rval += s.PowerRatingWatts
			}
			return rval
		}(),
	}
	return &fi, nil
}

type NetworkInterfaceDetails struct {
	Transport      powerwall.NetworkInterface
	Name           string
	Active         bool
	Enabled        bool
	Primary        bool
	SignalStrength int
}

type MeterType int

const (
	Total MeterType = iota
	Load
	Solar
	Battery
)

func (m MeterType) String() string {
	switch m {
	case Total:
		return "site"
	case Load:
		return "load"
	case Solar:
		return "solar"
	case Battery:
		return "battery"
	default:
		return "unknown"
	}
}

type MeterDetails struct {
	InstantPower          float64
	InstantReactivePower  float64
	InstantApparentPower  float64
	CumulativeEnergyTo    float64
	CumulativeEnergyFrom  float64
	InstantAverageVoltage float64
	InstantTotalCurrent   float64
}

type SoftwareVersion struct {
	Major, Minor, Release int64
}

type TeslaEnergyGatewayMetrics struct {
	Fixed FixedInfo
	// from operation:
	Mode                 powerwall.OperatingMode
	BackupReservePercent float64
	// from status:
	Uptime            time.Duration
	Version           SoftwareVersion
	NetworkInterfaces map[powerwall.NetworkInterface]NetworkInterfaceDetails
	// sitemaster
	SiteMasterRunning          bool
	SiteMasterConnectedToTesla bool
	SiteMasterSupplyingPower   bool
	Meters                     map[MeterType]MeterDetails
	// from soe:
	PowerwallChargePercent float64
	// from gridstatus:
	GridConnected bool
	GridActive    bool
}

var versionRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func (p *TeslaEnergyGatewayMetrics) getOperations(mon powerwall.Monitor) error {
	operation, err := mon.GetOperation()
	if err != nil {
		return err
	}
	p.Mode = operation.RealMode
	p.BackupReservePercent = operation.BackupReservePercent
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getStatus(mon powerwall.Monitor) error {
	status, err := mon.GetStatus()
	if err != nil {
		return err
	}
	p.Uptime = status.UpTime.Duration()
	versionParts := versionRegex.FindStringSubmatch(status.Version)
	if len(versionParts) != 4 {
		return fmt.Errorf("version %q unexpected, want A.B.C", status.Version)
	}
	p.Version.Major, err = strconv.ParseInt(versionParts[1], 10, 64)
	if err != nil {
		return err
	}
	p.Version.Minor, err = strconv.ParseInt(versionParts[2], 10, 64)
	if err != nil {
		return err
	}
	p.Version.Release, err = strconv.ParseInt(versionParts[3], 10, 64)
	if err != nil {
		return err
	}
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getNetworks(mon powerwall.Monitor) error {
	networks, err := mon.GetNetworks()
	if err != nil {
		return err
	}
	p.NetworkInterfaces = make(map[powerwall.NetworkInterface]NetworkInterfaceDetails)
	for _, nw := range networks {
		p.NetworkInterfaces[nw.Interface] = NetworkInterfaceDetails{
			Transport:      nw.Interface,
			Name:           nw.Name,
			Enabled:        nw.Enabled,
			Active:         nw.Active,
			Primary:        nw.Primary,
			SignalStrength: nw.Info.SignalStrength,
		}
	}
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getSiteMaster(mon powerwall.Monitor) error {
	siteMaster, err := mon.GetSiteMaster()
	if err != nil {
		return err
	}
	p.SiteMasterRunning = siteMaster.Running
	p.SiteMasterConnectedToTesla = siteMaster.ConnectedToTesla
	p.SiteMasterSupplyingPower = siteMaster.PowerSupplyMode
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getAggregates(mon powerwall.Monitor) error {
	p.Meters = make(map[MeterType]MeterDetails)
	agg, err := mon.GetAggregates()
	if err != nil {
		return err
	}
	getdetails := func(d powerwall.MeterDetails) MeterDetails {
		return MeterDetails{
			InstantPower:          d.InstantPower,
			InstantReactivePower:  d.InstantReactivePower,
			InstantApparentPower:  d.InstantApparentPower,
			CumulativeEnergyFrom:  d.EnergyExported,
			CumulativeEnergyTo:    d.EnergyImported,
			InstantAverageVoltage: d.InstantAverageVoltage,
			InstantTotalCurrent:   d.InstantTotalCurrent,
		}
	}
	p.Meters[Total] = getdetails(agg.Site)
	p.Meters[Load] = getdetails(agg.Load)
	p.Meters[Solar] = getdetails(agg.Solar)
	p.Meters[Battery] = getdetails(agg.Battery)
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getSOE(mon powerwall.Monitor) error {
	soe, err := mon.GetSOE()
	if err != nil {
		return err
	}
	p.PowerwallChargePercent = soe.Percentage

	gridstatus, err := mon.GetGridStatus()
	if err != nil {
		return err
	}
	p.GridActive = gridstatus.Active
	p.GridConnected = gridstatus.Status == powerwall.GridConnected
	return nil
}

func (p *TeslaEnergyGatewayMetrics) getDynamicInfo(fixed *FixedInfo, mon powerwall.Monitor) error {
	p.Fixed = *fixed
	ops := []func(mon powerwall.Monitor) error{
		p.getOperations,
		p.getStatus,
		p.getNetworks,
		p.getSiteMaster,
		p.getAggregates,
		p.getSOE,
	}
	for _, op := range ops {
		if err := op(mon); err != nil {
			return err
		}
	}
	return nil
}

// New retrieves fixed fields from an energy gateway.
func New(mon powerwall.Monitor) (*FixedInfo, error) {
	return fetchFixedInfo(mon)
}

// Poll retrieves dynamic fields from an energy gateway.
func Poll(mon powerwall.Monitor, fixed *FixedInfo) (*TeslaEnergyGatewayMetrics, error) {
	r := &TeslaEnergyGatewayMetrics{}
	if err := r.getDynamicInfo(fixed, mon); err != nil {
		return nil, err
	}
	return r, nil
}
