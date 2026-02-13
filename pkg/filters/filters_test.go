package filters

import (
	"testing"

	"Find-Meraki-Ports-With-MAC/pkg/meraki"
)

func TestFilterSwitches(t *testing.T) {
	devices := []meraki.Device{
		{Serial: "MS01", Model: "MS120", ProductType: "switch"},
		{Serial: "MS02", Model: "MS250", ProductType: "switch"},
		{Serial: "C901", Model: "C9300", ProductType: "switch"},
		{Serial: "MR01", Model: "MR44", ProductType: "wireless"},
		{Serial: "MX01", Model: "MX84", ProductType: "appliance"},
	}

	switches := FilterSwitches(devices)

	if len(switches) != 3 {
		t.Errorf("FilterSwitches() returned %d switches, want 3", len(switches))
	}

	for _, s := range switches {
		if s.ProductType != "switch" {
			t.Errorf("FilterSwitches() included non-switch device: %v", s)
		}
	}
}

func TestFilterSwitchesByName(t *testing.T) {
	devices := []meraki.Device{
		{Serial: "S1", Name: "core-switch-1", Model: "MS250"},
		{Serial: "S2", Name: "access-switch-2", Model: "MS120"},
		{Serial: "S3", Name: "distribution-switch-3", Model: "C9300"},
	}

	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{
			name:   "filter core",
			filter: "core",
			want:   1,
		},
		{
			name:   "filter switch (all match)",
			filter: "switch",
			want:   3,
		},
		{
			name:   "case insensitive",
			filter: "CORE",
			want:   1,
		},
		{
			name:   "no match",
			filter: "router",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterSwitchesByName(devices, tt.filter)
			if len(filtered) != tt.want {
				t.Errorf("FilterSwitchesByName(%q) returned %d devices, want %d", tt.filter, len(filtered), tt.want)
			}
		})
	}
}

func TestMatchesSwitchFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   bool
	}{
		{name: "core-switch-1", filter: "core", want: true},
		{name: "core-switch-1", filter: "CORE", want: true},
		{name: "core-switch-1", filter: "switch", want: true},
		{name: "core-switch-1", filter: "router", want: false},
		{name: "UPPERCASE", filter: "upper", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.filter, func(t *testing.T) {
			got := MatchesSwitchFilter(tt.name, tt.filter)
			if got != tt.want {
				t.Errorf("MatchesSwitchFilter(%q, %q) = %v, want %v", tt.name, tt.filter, got, tt.want)
			}
		})
	}
}

func TestMatchesPortFilter(t *testing.T) {
	tests := []struct {
		port   string
		filter string
		want   bool
	}{
		{port: "3", filter: "3", want: true},
		{port: "10", filter: "3", want: false},
		{port: "GigabitEthernet1/0/3", filter: "3", want: true},
		{port: "Gi1/0/3", filter: "3", want: true},
		{port: "port-3", filter: "3", want: true},
		{port: "4", filter: "3", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.port+"_"+tt.filter, func(t *testing.T) {
			got := MatchesPortFilter(tt.port, tt.filter)
			if got != tt.want {
				t.Errorf("MatchesPortFilter(%q, %q) = %v, want %v", tt.port, tt.filter, got, tt.want)
			}
		})
	}
}
