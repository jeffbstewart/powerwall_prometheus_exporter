package powerwall

import (
	"encoding/json"
	"fmt"
)

type NetworkInterface int

const (
	Ethernet NetworkInterface = iota
	Cellular
	Wifi
)

var stringToInterface = map[string]NetworkInterface{
	"EthType":  Ethernet,
	"GsmType":  Cellular,
	"WifiType": Wifi,
}

func (n NetworkInterface) String() string {
	switch n {
	case Ethernet:
		return "ethernet"
	case Cellular:
		return "cellular"
	case Wifi:
		return "wifi"
	default:
		return "unknownNetworkType"
	}
}

func (n *NetworkInterface) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	var ok bool
	*n, ok = stringToInterface[s]
	if !ok {
		return fmt.Errorf("unknown NetworkInterface %q", s)
	}
	return nil
}

type OperatingMode int

const (
	Backup OperatingMode = iota
	SelfConsumption
	Autonomous
	Scheduler
	SiteControl
)

func (o OperatingMode) String() string {
	switch o {
	case Backup:
		return "Backup"
	case SelfConsumption:
		return "Self Consumption"
	case Autonomous:
		return "Autonomous"
	case Scheduler:
		return "Scheduler"
	case SiteControl:
		return "SiteControl"
	default:
		return "UnknownOperatingMode"
	}
}

var stringToOperatingMode = map[string]OperatingMode{
	"backup":           Backup,
	"self_consumption": SelfConsumption,
	"autonomous":       Autonomous,
	"scheduler":        Scheduler,
	"site_control":     SiteControl,
}

func (n *OperatingMode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	var ok bool
	*n, ok = stringToOperatingMode[s]
	if !ok {
		return fmt.Errorf("unknown OperatingMode %q", s)
	}
	return nil
}

type SystemStatus int

const (
	GridConnected SystemStatus = iota
	IslandedReady
	IslandedActive
	TransitionToGrid
)

func (s SystemStatus) String() string {
	switch s {
	case GridConnected:
		return "GridConnected"
	case IslandedReady:
		return "IslandedReady"
	case IslandedActive:
		return "IslandedActive"
	case TransitionToGrid:
		return "TransitionToGrid"
	default:
		return "UnknownSystemStatus"
	}
}

var stringToSystemStatus = map[string]SystemStatus{
	"SystemGridConnected":    GridConnected,
	"SystemIslandedReady":    IslandedReady,
	"SystemIslandedActive":   IslandedActive,
	"SystemTransitionToGrid": TransitionToGrid,
}

func (s *SystemStatus) UnmarshalJSON(b []byte) error {
	var j string
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	var ok bool
	*s, ok = stringToSystemStatus[j]
	if !ok {
		return fmt.Errorf("unknown SystemStatus %q", j)
	}
	return nil
}

type GridState int

const (
	Compliant GridState = iota
	Qualifying
	Uncompliant
)

var stringToGridState = map[string]GridState{
	"Grid_Compliant":   Compliant,
	"Grid_Qualifying":  Qualifying,
	"Grid_Uncompliant": Uncompliant,
}

func (g GridState) String() string {
	switch g {
	case Compliant:
		return "Complaint"
	case Qualifying:
		return "Qualifying"
	case Uncompliant:
		return "Uncompliant"
	default:
		return "UnknownGridState"
	}
}

func (g *GridState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	var ok bool
	*g, ok = stringToGridState[s]
	if !ok {
		return fmt.Errorf("unknown GridState %q", s)
	}
	return nil
}

// jrester code suggets values here include:
// Grid_Compliant, Grid_Qualifying, Grid_Uncompliant
