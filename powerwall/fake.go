package powerwall

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// NewFake returns a Monitor backed by recorded gateway responses.  It
// exercises the same JSON decoding paths as the real monitor, so tests
// built on it are hermetic yet faithful to the wire format.
func NewFake() Monitor {
	return fakeMonitor{}
}

type fakeMonitor struct{}

func (fakeMonitor) Close() error {
	return nil
}

func decodeFake(endpoint, recorded string, into interface{}) error {
	if err := json.Unmarshal([]byte(recorded), into); err != nil {
		return fmt.Errorf("decode recorded response for %s: %v", endpoint, err)
	}
	return nil
}

const fakeNetworksJSON = `[
  {
    "network_name": "ethernet_tesla_internal_default",
    "interface": "EthType",
    "dhcp": true,
    "enabled": true,
    "active": true,
    "primary": true,
    "iface_network_info": {
      "network_name": "ethernet_tesla_internal_default",
      "networks": [{"ip": "192.168.7.201", "netmask": 24}],
      "gateway": "192.168.7.1",
      "interface": "EthType",
      "state": "activated",
      "state_reason": "",
      "signal_strength": 0,
      "hw_address": "98:ed:5c:aa:bb:cc"
    }
  },
  {
    "network_name": "wifi_tesla_internal",
    "interface": "WifiType",
    "dhcp": true,
    "enabled": true,
    "active": false,
    "primary": false,
    "iface_network_info": {
      "network_name": "wifi_tesla_internal",
      "networks": [],
      "gateway": "",
      "interface": "WifiType",
      "state": "disconnected",
      "state_reason": "",
      "signal_strength": 0,
      "hw_address": "98:ed:5c:aa:bb:cd"
    }
  },
  {
    "network_name": "cellular_tesla_internal",
    "interface": "GsmType",
    "dhcp": false,
    "enabled": false,
    "active": false,
    "primary": false,
    "iface_network_info": {
      "network_name": "cellular_tesla_internal",
      "networks": [],
      "gateway": "",
      "interface": "GsmType",
      "state": "disconnected",
      "state_reason": "",
      "signal_strength": 13,
      "hw_address": ""
    }
  }
]`

func (fakeMonitor) GetNetworks() ([]Network, error) {
	var resp []Network
	if err := decodeFake("/networks", fakeNetworksJSON, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

const fakeSiteInfoJSON = `{
  "max_system_energy_kWh": 0,
  "max_system_power_kW": 0,
  "site_name": "Fake Home",
  "timezone": "America/New_York",
  "max_site_meter_power_kW": 1000000000,
  "min_site_meter_power_kW": -1000000000,
  "nominal_system_energy_kWh": 27,
  "nominal_system_power_kW": 10.8,
  "grid_code": {
    "grid_code": "60Hz_240V_s_UL1741SA:2018_ISO-NE",
    "grid_voltage_setting": 240,
    "grid_freq_setting": 60,
    "grid_phase_setting": "Split",
    "country": "United States",
    "state": "Massachusetts",
    "distributor": "*",
    "utility": "Eversource Energy (NSTAR-Cambridge Electric Light)",
    "retailer": "*",
    "region": "UL1741SA-ISO-NE:2018"
  }
}`

func (fakeMonitor) GetSiteInfo() (*SiteInfo, error) {
	var resp SiteInfo
	if err := decodeFake("/site_info", fakeSiteInfoJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeOperationJSON = `{
  "real_mode": "self_consumption",
  "backup_reserve_percent": 20.5,
  "freq_shift_load_shed_soe": 65,
  "freq_shift_load_shed_delta_f": -0.32
}`

func (fakeMonitor) GetOperation() (*Operation, error) {
	var resp Operation
	if err := decodeFake("/operation", fakeOperationJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeConfigJSON = `{"vin": "1232100-00-E--TG123456789012"}`

func (fakeMonitor) GetConfig() (*Config, error) {
	var resp Config
	if err := decodeFake("/config", fakeConfigJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakePowerwallsJSON = `{
  "enumerating": false,
  "updating": false,
  "checking_if_offgrid": false,
  "running_phase_detection": false,
  "phase_detection_last_error": "no phase information",
  "bubble_shedding": false,
  "on_grid_check_error": "on grid check not run",
  "grid_qualifying": false,
  "grid_code_validating": false,
  "phase_detection_not_available": true,
  "powerwalls": [
    {
      "Type": "",
      "PackagePartNumber": "2012170-25-E",
      "PackageSerialNumber": "TG000000000001",
      "type": "acpw",
      "grid_state": "Grid_Compliant",
      "grid_reconnection_time_seconds": 0,
      "under_phase_detection": false,
      "updating": false,
      "commissioning_diagnostic": {
        "name": "Commissioning",
        "category": "InternalComms",
        "disruptive": false,
        "checks": []
      },
      "update_diagnostic": {
        "name": "Firmware Update",
        "category": "InternalComms",
        "disruptive": true,
        "checks": []
      }
    },
    {
      "Type": "",
      "PackagePartNumber": "2012170-25-E",
      "PackageSerialNumber": "TG000000000002",
      "type": "acpw",
      "grid_state": "Grid_Compliant",
      "grid_reconnection_time_seconds": 0,
      "under_phase_detection": false,
      "updating": false,
      "commissioning_diagnostic": {
        "name": "Commissioning",
        "category": "InternalComms",
        "disruptive": false,
        "checks": []
      },
      "update_diagnostic": {
        "name": "Firmware Update",
        "category": "InternalComms",
        "disruptive": true,
        "checks": []
      }
    }
  ]
}`

func (fakeMonitor) GetPowerwalls() (*Powerwalls, error) {
	var resp Powerwalls
	if err := decodeFake("/powerwalls", fakePowerwallsJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeStatusJSON = `{
  "start_time": "2026-08-01 10:15:30 -0400",
  "up_time_seconds": "143h54m32.539257895s",
  "is_new": false,
  "version": "23.44.3 eb113390",
  "git_hash": "eb11339022a01e0b9e26bd6ecaebbcf237bbb6ca",
  "commission_count": 0,
  "device_type": "teg",
  "sync_type": "v2.1"
}`

func (fakeMonitor) GetStatus() (*Status, error) {
	var resp Status
	if err := decodeFake("/status", fakeStatusJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeSiteMasterJSON = `{
  "status": "StatusUp",
  "running": true,
  "connected_to_tesla": true,
  "power_supply_mode": false
}`

func (fakeMonitor) GetSiteMaster() (*SiteMaster, error) {
	var resp SiteMaster
	if err := decodeFake("/sitemaster", fakeSiteMasterJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeAggregatesJSON = `{
  "site": {
    "last_communication_time": "2026-08-01T10:15:30.269827072-04:00",
    "instant_power": 743.5,
    "instant_reactive_power": -401.2,
    "instant_apparant_power": 845.1,
    "frequency": 60.02,
    "energy_exported": 4744838.61,
    "energy_imported": 12464566.55,
    "instant_average_voltage": 246.35,
    "instant_total_current": 3.02,
    "i_a_current": 0,
    "i_b_current": 0,
    "i_c_current": 0,
    "last_phase_voltage_communication_time": "",
    "last_phase_power_communication_time": "",
    "timeout": 1500000000
  },
  "battery": {
    "last_communication_time": "2026-08-01T10:15:30.269827072-04:00",
    "instant_power": -2350,
    "instant_reactive_power": 20.5,
    "instant_apparant_power": 2350.09,
    "frequency": 60,
    "energy_exported": 5124890.2,
    "energy_imported": 6156780.9,
    "instant_average_voltage": 245.9,
    "instant_total_current": -9.6,
    "i_a_current": 0,
    "i_b_current": 0,
    "i_c_current": 0,
    "last_phase_voltage_communication_time": "",
    "last_phase_power_communication_time": "",
    "timeout": 1500000000
  },
  "load": {
    "last_communication_time": "2026-08-01T10:15:30.269827072-04:00",
    "instant_power": 1893.75,
    "instant_reactive_power": -180.6,
    "instant_apparant_power": 1902.35,
    "frequency": 60.02,
    "energy_exported": 0,
    "energy_imported": 25873109.44,
    "instant_average_voltage": 246.1,
    "instant_total_current": 7.7,
    "i_a_current": 0,
    "i_b_current": 0,
    "i_c_current": 0,
    "last_phase_voltage_communication_time": "",
    "last_phase_power_communication_time": "",
    "timeout": 1500000000
  },
  "solar": {
    "last_communication_time": "2026-08-01T10:15:30.269827072-04:00",
    "instant_power": 3500.25,
    "instant_reactive_power": -50.4,
    "instant_apparant_power": 3500.61,
    "frequency": 60.01,
    "energy_exported": 31819300.4,
    "energy_imported": 1305.7,
    "instant_average_voltage": 246.4,
    "instant_total_current": 14.2,
    "i_a_current": 0,
    "i_b_current": 0,
    "i_c_current": 0,
    "last_phase_voltage_communication_time": "",
    "last_phase_power_communication_time": "",
    "timeout": 1500000000
  }
}`

func (fakeMonitor) GetAggregates() (*Aggregates, error) {
	var resp Aggregates
	if err := decodeFake("/meters/aggregates", fakeAggregatesJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeSOEJSON = `{"percentage": 87.5}`

func (fakeMonitor) GetSOE() (*SOE, error) {
	var resp SOE
	if err := decodeFake("/system_status/soe", fakeSOEJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeGridStatusJSON = `{
  "grid_status": "SystemGridConnected",
  "grid_services_active": false
}`

func (fakeMonitor) GetGridStatus() (*GridStatus, error) {
	var resp GridStatus
	if err := decodeFake("/system_status/grid_status", fakeGridStatusJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeSolarsJSON = `[
  {
    "brand": "SolarEdge Technologies",
    "model": "SE7600H-US (240V)",
    "power_rating_watts": 7600
  },
  {
    "brand": "SolarEdge Technologies",
    "model": "SE3800H-US (240V)",
    "power_rating_watts": 3800
  }
]`

func (fakeMonitor) GetSolars() ([]Solar, error) {
	var resp []Solar
	if err := decodeFake("/solars", fakeSolarsJSON, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

const fakeInstallerJSON = `{
  "company": "Fake Solar Co",
  "customer_id": "",
  "phone": "",
  "email": "",
  "location": "",
  "mounting": "",
  "wiring": "",
  "backup_configuration": "Whole Home",
  "solar_installation": "New",
  "has_stack_kit": false,
  "has_powerline_to_ethernet": false,
  "run_sitemaster": true,
  "verified_config": true,
  "installation_types": ["Solar", "Storage"]
}`

func (fakeMonitor) GetInstaller() (*Installer, error) {
	var resp Installer
	if err := decodeFake("/installer", fakeInstallerJSON, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

const fakeLoginResponseJSON = `{
  "email": "fake@example.com",
  "firstname": "Tesla",
  "lastname": "Energy",
  "roles": ["Home_Owner"],
  "token": "fake-token",
  "provider": "Basic",
  "loginTime": "2026-08-01T10:15:30.000000000-04:00"
}`

// NewFakeGatewayHandler serves the same recorded responses as NewFake
// over the gateway's HTTP API, so the real client in this package can
// be exercised end to end without a Tesla Energy Gateway.
func NewFakeGatewayHandler() http.Handler {
	mux := http.NewServeMux()
	serve := func(body string) http.HandlerFunc {
		return func(rw http.ResponseWriter, _ *http.Request) {
			rw.Header().Set("Content-Type", "application/json")
			if _, err := rw.Write([]byte(body)); err != nil {
				log.Printf("ERROR: fake gateway write: %v", err)
			}
		}
	}
	mux.HandleFunc("/api/login/Basic", serve(fakeLoginResponseJSON))
	mux.HandleFunc("/api/networks", serve(fakeNetworksJSON))
	mux.HandleFunc("/api/site_info", serve(fakeSiteInfoJSON))
	mux.HandleFunc("/api/operation", serve(fakeOperationJSON))
	mux.HandleFunc("/api/config", serve(fakeConfigJSON))
	mux.HandleFunc("/api/powerwalls", serve(fakePowerwallsJSON))
	mux.HandleFunc("/api/status", serve(fakeStatusJSON))
	mux.HandleFunc("/api/sitemaster", serve(fakeSiteMasterJSON))
	mux.HandleFunc("/api/meters/aggregates", serve(fakeAggregatesJSON))
	mux.HandleFunc("/api/system_status/soe", serve(fakeSOEJSON))
	mux.HandleFunc("/api/system_status/grid_status", serve(fakeGridStatusJSON))
	mux.HandleFunc("/api/solars", serve(fakeSolarsJSON))
	mux.HandleFunc("/api/installer", serve(fakeInstallerJSON))
	return mux
}
