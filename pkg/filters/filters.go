// Package filters provides utilities for filtering network devices and ports.
package filters

import (
	"strings"

	"Find-Meraki-Ports-With-MAC/pkg/meraki"
)

// FilterSwitches returns only devices that are switches.
// A device is considered a switch if:
// - productType is "switch", OR
// - model starts with "MS" (Meraki switches), OR
// - model starts with "C9" (Catalyst 9000 series)
func FilterSwitches(devices []meraki.Device) []meraki.Device {
	var switches []meraki.Device
	for _, d := range devices {
		if d.ProductType == "switch" {
			switches = append(switches, d)
			continue
		}
		if strings.HasPrefix(strings.ToUpper(d.Model), "MS") {
			switches = append(switches, d)
			continue
		}
		if strings.HasPrefix(strings.ToUpper(d.Model), "C9") {
			switches = append(switches, d)
			continue
		}
	}
	return switches
}

// FilterSwitchesByName filters devices by a case-insensitive substring match on the name.
func FilterSwitchesByName(devices []meraki.Device, filter string) []meraki.Device {
	if filter == "" {
		return devices
	}
	var filtered []meraki.Device
	for _, d := range devices {
		if MatchesSwitchFilter(d.Name, filter) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// MatchesSwitchFilter checks if a switch name matches the filter (case-insensitive substring).
func MatchesSwitchFilter(name, filter string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(filter))
}

// MatchesPortFilter checks if a port matches the filter.
// The filter can be an exact match or a substring match.
func MatchesPortFilter(port, filter string) bool {
	if port == filter {
		return true
	}
	return strings.Contains(port, filter)
}
