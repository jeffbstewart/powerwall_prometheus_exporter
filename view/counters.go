package view

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/model"
	"github.com/jeffbstewart/powerwall_prometheus_exporter/powerwall"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
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

func New(fixed *model.FixedInfo, opts Options) (*PrometheusCounters, error) {
	ss, ns := opts.Subsystem, opts.Namespace
	r := &PrometheusCounters{
		powerwallChargePercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "powerwall_charge_percent",
			Help:      "percent of nominal powerwall power available for supply generation",
		}),
		nominalSystemEnergykWh: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "nominal_system_energy_kWh",
			Help:      "nominal rated energy that can be delivered by the inverter.",
		}),
		nominalSystemPowerkW: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "nominal_system_power_kW",
			Help:      "nominal rated power that can be delivered by the inverter.",
		}),
		numPowerwalls: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "num_powerwalls",
			Help:      "Number of powerwall battery systems managed by the energy gateway",
		}),
		totalSolarRatingWatts: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "total_solar_rating_W",
			Help:      "rated total power output of all solar arrays connected to the inverter",
		}),
		backupMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "operating_in_backup_only_mode",
			Help:      "if 1, the powerwalls are only consumed for backup power",
		}),
		selfConsumptionMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "operating_in_self_consumption_mode",
			Help:      "if 1, the powerwalls cycle between charging and discharing",
		}),
		backupReservePercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "backup_reserve_percent",
			Help:      "Percent of battery capacity not used unless the grid is out",
		}),
		uptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "uptime_seconds",
			Help:      "Runtime of the Tesla energy gateway",
		}),
		majorVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "major_version",
			Help:      "The major version of the software in the Tesla energy gateway.  In version 1.2.3, the major version is the 1",
		}),
		minorVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "minor_version",
			Help:      "The minor version of the software in the Telsa energy gateway.  In version 1.2.3, the minor version is the 2",
		}),
		releaseVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "release_version",
			Help:      "The release version of the software in the Tesla energy gateway.  In version 1.2.3, the release version is the 3",
		}),
		flattenedVersion: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "flattened_version",
			Help:      "The version of the software in the Tesla energy gateway, flattened.  Version 10.12.7 would be 10127",
		}),
		networkActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "network_active",
			Help:      "if 1, the given network interface appears to be usable",
		}, []string{kInterface}),
		networkEnabled: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "network_enabled",
			Help:      "if 1, the given network interface is administratively enabled",
		}, []string{kInterface}),
		networkPrimary: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "network_primary",
			Help:      "if 1, the given network interface is the preferred interface",
		}, []string{kInterface}),
		networkSignalStrength: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "network_signal_strength",
			Help:      "signal to noise ratio in dB for the interface.  Only populated for cellular",
		}, []string{kInterface}),
		siteMasterRunning: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "sitemaster_running",
			Help:      "if 1, the site master is running",
		}),
		siteMasterConnectedToTesla: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "site_master_connected_to_tesla",
			Help:      "if 1, the site master can communicate with Tesla",
		}),
		siteMasterSupplyingPower: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "site_master_supplying_power",
			Help:      "if 1, the site master is supplying power instead of the grid",
		}),
		instantPower: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "instant_power",
			Help:      "power measured by the given meter at a moment in time",
		}, []string{kMeter, kPowerType}),
		cumulativePower: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "cumulative_power",
			Help:      "cumulative power measured over the lifetime of the given meter, in units of kWh",
		}, []string{kMeter, kDirection}),
		instantAverageVoltage: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "instant_average_voltage",
			Help:      "electrical potential measured by the given meter at a moment in time, in units of volts",
		}, []string{kMeter}),
		instantTotalCurrent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "instant_total_current_amps",
			Help:      "electrical current measured by the given meter at a moment in time, in units of amperes",
		}, []string{kMeter}),
		gridConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "grid_connected",
			Help:      "if 1, the grid is available to supply power",
		}),
		gridActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: ss,
			Name:      "grid_active",
			Help:      "if 1, the grid is actively supplying power",
		}),
	}
	r.nominalSystemEnergykWh.Set(fixed.NominalSystemEnergykWh)
	r.nominalSystemPowerkW.Set(fixed.NominalSystemPowerkW)
	r.numPowerwalls.Set(float64(fixed.NumPowerwalls))
	r.totalSolarRatingWatts.Set(float64(fixed.TotalSolarPowerRatingWatts))

	cols := []prometheus.Collector{
		r.powerwallChargePercent,
		r.nominalSystemEnergykWh,
		r.nominalSystemPowerkW,
		r.numPowerwalls,
		r.totalSolarRatingWatts,
		r.backupMode,
		r.selfConsumptionMode,
		r.backupReservePercent,
		r.uptimeSeconds,
		r.majorVersion,
		r.minorVersion,
		r.releaseVersion,
		r.flattenedVersion,
		r.networkActive,
		r.networkEnabled,
		r.networkPrimary,
		r.networkSignalStrength,
		r.siteMasterRunning,
		r.siteMasterConnectedToTesla,
		r.siteMasterSupplyingPower,
		r.instantPower,
		r.cumulativePower,
		r.instantAverageVoltage,
		r.instantTotalCurrent,
		r.gridConnected,
		r.gridActive,
	}
	for _, c := range cols {
		if err := prometheus.Register(c); err != nil {
			return nil, err
		}
	}
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
	powerwallChargePercent     prometheus.Gauge
	nominalSystemEnergykWh     prometheus.Gauge
	nominalSystemPowerkW       prometheus.Gauge
	numPowerwalls              prometheus.Gauge
	totalSolarRatingWatts      prometheus.Gauge
	backupMode                 prometheus.Gauge
	selfConsumptionMode        prometheus.Gauge
	backupReservePercent       prometheus.Gauge
	uptimeSeconds              prometheus.Gauge
	majorVersion               prometheus.Gauge
	minorVersion               prometheus.Gauge
	releaseVersion             prometheus.Gauge
	flattenedVersion           prometheus.Gauge
	networkActive              *prometheus.GaugeVec
	networkEnabled             *prometheus.GaugeVec
	networkPrimary             *prometheus.GaugeVec
	networkSignalStrength      *prometheus.GaugeVec
	siteMasterRunning          prometheus.Gauge
	siteMasterConnectedToTesla prometheus.Gauge
	siteMasterSupplyingPower   prometheus.Gauge
	instantPower               *prometheus.GaugeVec
	priorCumulative            map[model.MeterType]map[string] /* direction*/ float64
	cumulativePower            *prometheus.CounterVec
	instantAverageVoltage      *prometheus.GaugeVec
	instantTotalCurrent        *prometheus.GaugeVec
	gridConnected              prometheus.Gauge
	gridActive                 prometheus.Gauge
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
		labels := prometheus.Labels{kInterface: net.Transport.String()}
		p.networkEnabled.With(labels).Set(boolToFloat(net.Enabled))
		p.networkActive.With(labels).Set(boolToFloat(net.Active))
		p.networkPrimary.With(labels).Set(boolToFloat(net.Primary))
		p.networkSignalStrength.With(labels).Set(float64(net.SignalStrength))
	}
	p.siteMasterRunning.Set(boolToFloat(m.SiteMasterRunning))
	p.siteMasterConnectedToTesla.Set(boolToFloat(m.SiteMasterConnectedToTesla))
	p.siteMasterSupplyingPower.Set(boolToFloat(m.SiteMasterSupplyingPower))
	for mt, meter := range m.Meters {
		p.instantPower.With(prometheus.Labels{kMeter: mt.String(), kPowerType: kTruePower}).Set(meter.InstantPower)
		p.instantPower.With(prometheus.Labels{kMeter: mt.String(), kPowerType: kReactivePower}).Set(meter.InstantReactivePower)
		p.instantPower.With(prometheus.Labels{kMeter: mt.String(), kPowerType: kApparentPower}).Set(meter.InstantApparentPower)
		labels := prometheus.Labels{kMeter: mt.String()}
		p.instantAverageVoltage.With(labels).Set(meter.InstantAverageVoltage)
		p.instantTotalCurrent.With(labels).Set(meter.InstantTotalCurrent)
		prior := p.priorCumulative[mt][kTo]
		delta := meter.CumulativeEnergyTo - prior
		p.priorCumulative[mt][kTo] = meter.CumulativeEnergyTo
		const epsilon = 0.00001
		if delta < 0 {
			if delta < -epsilon {
				glog.Warningf("Meter %s cumulative energy to decreased: %.4f", mt, delta)
			}
		} else {
			p.cumulativePower.With(prometheus.Labels{
				kMeter:     mt.String(),
				kDirection: kTo,
			}).Add(delta)
		}
		prior = p.priorCumulative[mt][kFrom]
		delta = meter.CumulativeEnergyFrom - prior
		if delta < 0 {
			if delta < -epsilon {
				glog.Warningf("Meter %s cumulative energy from decreased: %.4f", mt, delta)
			}
		} else {
			p.cumulativePower.With(prometheus.Labels{
				kMeter:     mt.String(),
				kDirection: kFrom,
			}).Add(delta)
		}
		p.priorCumulative[mt][kFrom] = meter.CumulativeEnergyFrom
	}
	p.gridConnected.Set(boolToFloat(m.GridConnected))
	p.gridActive.Set(boolToFloat(m.GridActive))
	return nil
}
