package view

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeffbstewart/powerwall_prometheus_exporter/internal/expo"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/model"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
)

// Options describes information needed to export metrics to Prometheus.
type Options struct {
	// Namespace is part of the Prometheus hierarchy of naming.  It does not
	// appear to affect the exported statistics.  Just set it to something.
	Namespace string
	// Subsystem is part of the Prometheus hierarchy of namign.  It does not
	// appear to affect the exported statistics.  Just set it to something.
	Subsystem string
}

const (
	kInterface     = "interface"
	kMeter         = "meter"
	kDirection     = "direction"
	kFrom          = "from"
	kTo            = "to"
	kPowerType     = "powerType"
	kTruePower     = "truePower"
	kReactivePower = "reactivePower"
	kApparentPower = "apparentPower"
)

// fqName joins namespace, subsystem, and name with underscores,
// skipping empty parts, the way client_golang builds metric names.
func fqName(namespace, subsystem, name string) string {
	var parts []string
	for _, p := range []string{namespace, subsystem, name} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "_")
}

func New(fixed *model.FixedInfo, opts Options) (*PrometheusCounters, error) {
	reg := expo.NewRegistry()
	var err error
	gauge := func(name, help string) *expo.Gauge {
		if err != nil {
			return nil
		}
		var g *expo.Gauge
		g, err = reg.NewGauge(fqName(opts.Namespace, opts.Subsystem, name), help)
		return g
	}
	gaugeVec := func(name, help string, labelNames ...string) *expo.GaugeVec {
		if err != nil {
			return nil
		}
		var g *expo.GaugeVec
		g, err = reg.NewGaugeVec(fqName(opts.Namespace, opts.Subsystem, name), help, labelNames...)
		return g
	}
	counterVec := func(name, help string, labelNames ...string) *expo.CounterVec {
		if err != nil {
			return nil
		}
		var c *expo.CounterVec
		c, err = reg.NewCounterVec(fqName(opts.Namespace, opts.Subsystem, name), help, labelNames...)
		return c
	}
	r := &PrometheusCounters{
		registry: reg,
		powerwallChargePercent: gauge("powerwall_charge_percent",
			"percent of nominal powerwall power available for supply generation"),
		nominalSystemEnergykWh: gauge("nominal_system_energy_kWh",
			"nominal rated energy that can be delivered by the inverter."),
		nominalSystemPowerkW: gauge("nominal_system_power_kW",
			"nominal rated power that can be delivered by the inverter."),
		numPowerwalls: gauge("num_powerwalls",
			"Number of powerwall battery systems managed by the energy gateway"),
		totalSolarRatingWatts: gauge("total_solar_rating_W",
			"rated total power output of all solar arrays connected to the inverter"),
		backupMode: gauge("operating_in_backup_only_mode",
			"if 1, the powerwalls are only consumed for backup power"),
		selfConsumptionMode: gauge("operating_in_self_consumption_mode",
			"if 1, the powerwalls cycle between charging and discharing"),
		backupReservePercent: gauge("backup_reserve_percent",
			"Percent of battery capacity not used unless the grid is out"),
		uptimeSeconds: gauge("uptime_seconds",
			"Runtime of the Tesla energy gateway"),
		majorVersion: gauge("major_version",
			"The major version of the software in the Tesla energy gateway.  In version 1.2.3, the major version is the 1"),
		minorVersion: gauge("minor_version",
			"The minor version of the software in the Telsa energy gateway.  In version 1.2.3, the minor version is the 2"),
		releaseVersion: gauge("release_version",
			"The release version of the software in the Tesla energy gateway.  In version 1.2.3, the release version is the 3"),
		flattenedVersion: gauge("flattened_version",
			"The version of the software in the Tesla energy gateway, flattened.  Version 10.12.7 would be 10127"),
		networkActive: gaugeVec("network_active",
			"if 1, the given network interface appears to be usable", kInterface),
		networkEnabled: gaugeVec("network_enabled",
			"if 1, the given network interface is administratively enabled", kInterface),
		networkPrimary: gaugeVec("network_primary",
			"if 1, the given network interface is the preferred interface", kInterface),
		networkSignalStrength: gaugeVec("network_signal_strength",
			"signal to noise ratio in dB for the interface.  Only populated for cellular", kInterface),
		siteMasterRunning: gauge("sitemaster_running",
			"if 1, the site master is running"),
		siteMasterConnectedToTesla: gauge("site_master_connected_to_tesla",
			"if 1, the site master can communicate with Tesla"),
		siteMasterSupplyingPower: gauge("site_master_supplying_power",
			"if 1, the site master is supplying power instead of the grid"),
		instantPower: gaugeVec("instant_power",
			"power measured by the given meter at a moment in time", kMeter, kPowerType),
		cumulativePower: counterVec("cumulative_power",
			"cumulative power measured over the lifetime of the given meter, in units of kWh", kMeter, kDirection),
		instantAverageVoltage: gaugeVec("instant_average_voltage",
			"electrical potential measured by the given meter at a moment in time, in units of volts", kMeter),
		instantTotalCurrent: gaugeVec("instant_total_current_amps",
			"electrical current measured by the given meter at a moment in time, in units of amperes", kMeter),
		gridConnected: gauge("grid_connected",
			"if 1, the grid is available to supply power"),
		gridActive: gauge("grid_active",
			"if 1, the grid is actively supplying power"),
	}
	if err != nil {
		return nil, err
	}
	r.nominalSystemEnergykWh.Set(fixed.NominalSystemEnergykWh)
	r.nominalSystemPowerkW.Set(fixed.NominalSystemPowerkW)
	r.numPowerwalls.Set(float64(fixed.NumPowerwalls))
	r.totalSolarRatingWatts.Set(float64(fixed.TotalSolarPowerRatingWatts))

	r.priorCumulative = make(map[model.MeterType]map[string]float64)
	for _, mt := range []model.MeterType{
		model.Total,
		model.Solar,
		model.Battery,
		model.Load,
	} {
		r.priorCumulative[mt] = make(map[string]float64)
	}
	return r, nil
}

type PrometheusCounters struct {
	registry                   *expo.Registry
	powerwallChargePercent     *expo.Gauge
	nominalSystemEnergykWh     *expo.Gauge
	nominalSystemPowerkW       *expo.Gauge
	numPowerwalls              *expo.Gauge
	totalSolarRatingWatts      *expo.Gauge
	backupMode                 *expo.Gauge
	selfConsumptionMode        *expo.Gauge
	backupReservePercent       *expo.Gauge
	uptimeSeconds              *expo.Gauge
	majorVersion               *expo.Gauge
	minorVersion               *expo.Gauge
	releaseVersion             *expo.Gauge
	flattenedVersion           *expo.Gauge
	networkActive              *expo.GaugeVec
	networkEnabled             *expo.GaugeVec
	networkPrimary             *expo.GaugeVec
	networkSignalStrength      *expo.GaugeVec
	siteMasterRunning          *expo.Gauge
	siteMasterConnectedToTesla *expo.Gauge
	siteMasterSupplyingPower   *expo.Gauge
	instantPower               *expo.GaugeVec
	priorCumulative            map[model.MeterType]map[string] /* direction*/ float64
	cumulativePower            *expo.CounterVec
	instantAverageVoltage      *expo.GaugeVec
	instantTotalCurrent        *expo.GaugeVec
	gridConnected              *expo.Gauge
	gridActive                 *expo.Gauge
}

// Handler serves the current metric values in the Prometheus text
// exposition format.
func (p *PrometheusCounters) Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		if err := p.registry.Render(&buf); err != nil {
			log.Printf("ERROR: rendering metrics: %v", err)
			rw.WriteHeader(500)
			return
		}
		rw.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if _, err := rw.Write(buf.Bytes()); err != nil {
			log.Printf("ERROR: writing metrics response: %v", err)
		}
	})
}

func (p *PrometheusCounters) Update(m *model.TeslaEnergyGatewayMetrics) error {
	p.powerwallChargePercent.Set(m.PowerwallChargePercent)
	if m.Mode == powerwall.Backup {
		p.backupMode.Set(1)
	} else {
		p.backupMode.Set(0)
	}
	if m.Mode == powerwall.SelfConsumption {
		p.selfConsumptionMode.Set(1)
	} else {
		p.selfConsumptionMode.Set(0)
	}
	// not sure what to do with Autonomous, Scheduler, or SiteControl.
	// Is Scheduler "use the power on this schedule" mode?
	// If so, that might make a useful export.
	p.backupReservePercent.Set(m.BackupReservePercent)
	p.uptimeSeconds.Set(float64(m.Uptime) / float64(time.Second))
	p.majorVersion.Set(float64(m.Version.Major))
	p.minorVersion.Set(float64(m.Version.Minor))
	p.releaseVersion.Set(float64(m.Version.Release))
	fs := fmt.Sprintf("%02d%02d%02d", m.Version.Major, m.Version.Minor, m.Version.Release)
	flat, err := strconv.ParseInt(fs, 10, 64)
	if err != nil {
		return err
	}
	p.flattenedVersion.Set(float64(flat))
	boolToFloat := func(b bool) float64 {
		if b {
			return 1
		}
		return 0
	}
	for _, net := range m.NetworkInterfaces {
		iface := net.Transport.String()
		p.networkEnabled.Set(boolToFloat(net.Enabled), iface)
		p.networkActive.Set(boolToFloat(net.Active), iface)
		p.networkPrimary.Set(boolToFloat(net.Primary), iface)
		p.networkSignalStrength.Set(float64(net.SignalStrength), iface)
	}
	p.siteMasterRunning.Set(boolToFloat(m.SiteMasterRunning))
	p.siteMasterConnectedToTesla.Set(boolToFloat(m.SiteMasterConnectedToTesla))
	p.siteMasterSupplyingPower.Set(boolToFloat(m.SiteMasterSupplyingPower))
	for mt, meter := range m.Meters {
		p.instantPower.Set(meter.InstantPower, mt.String(), kTruePower)
		p.instantPower.Set(meter.InstantReactivePower, mt.String(), kReactivePower)
		p.instantPower.Set(meter.InstantApparentPower, mt.String(), kApparentPower)
		p.instantAverageVoltage.Set(meter.InstantAverageVoltage, mt.String())
		p.instantTotalCurrent.Set(meter.InstantTotalCurrent, mt.String())
		prior := p.priorCumulative[mt][kTo]
		delta := meter.CumulativeEnergyTo - prior
		p.priorCumulative[mt][kTo] = meter.CumulativeEnergyTo
		const epsilon = 0.00001
		if delta < 0 {
			if delta < -epsilon {
				log.Printf("WARN: Meter %s cumulative energy to decreased: %.4f", mt, delta)
			}
		} else {
			p.cumulativePower.Add(delta, mt.String(), kTo)
		}
		prior = p.priorCumulative[mt][kFrom]
		delta = meter.CumulativeEnergyFrom - prior
		if delta < 0 {
			if delta < -epsilon {
				log.Printf("WARN: Meter %s cumulative energy from decreased: %.4f", mt, delta)
			}
		} else {
			p.cumulativePower.Add(delta, mt.String(), kFrom)
		}
		p.priorCumulative[mt][kFrom] = meter.CumulativeEnergyFrom
	}
	p.gridConnected.Set(boolToFloat(m.GridConnected))
	p.gridActive.Set(boolToFloat(m.GridActive))
	return nil
}
