// Copyright (C) 2025 Kent Behrends
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"testing"
)

func TestParseAggrPort_Resolve(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantClean  string
		wantMember []string
	}{
		{
			name:       "plain port unchanged",
			raw:        "42",
			wantClean:  "42",
			wantMember: nil,
		},
		{
			name:       "AGGR without member list",
			raw:        "AGGR/1",
			wantClean:  "AGGR/1",
			wantMember: nil,
		},
		{
			name:       "AGGR with embedded member list",
			raw:        "AGGR/0=98:18:88:63:BA:37/49,98:18:88:63:BA:37/50,98:18:88:63:BA:37/52",
			wantClean:  "AGGR/0",
			wantMember: []string{"49", "50", "52"},
		},
		{
			name:       "AGGR single member",
			raw:        "AGGR/2=Q2HP-ABCD/23",
			wantClean:  "AGGR/2",
			wantMember: []string{"23"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clean, members := parseAggrPort(tc.raw)
			if clean != tc.wantClean {
				t.Errorf("parseAggrPort(%q) clean = %q, want %q", tc.raw, clean, tc.wantClean)
			}
			if len(members) != len(tc.wantMember) {
				t.Errorf("parseAggrPort(%q) members = %v, want %v", tc.raw, members, tc.wantMember)
				return
			}
			for i := range members {
				if members[i] != tc.wantMember[i] {
					t.Errorf("parseAggrPort(%q) members[%d] = %q, want %q", tc.raw, i, members[i], tc.wantMember[i])
				}
			}
		})
	}
}

func TestIsPortUplink(t *testing.T) {
	tests := []struct {
		name        string
		portID      string
		aggrMembers []string
		uplinkSet   map[string]struct{}
		want        bool
	}{
		{
			name:      "direct uplink port",
			portID:    "49",
			uplinkSet: map[string]struct{}{"49": {}},
			want:      true,
		},
		{
			name:        "AGGR member is uplink",
			portID:      "AGGR/0",
			aggrMembers: []string{"49", "50"},
			uplinkSet:   map[string]struct{}{"49": {}},
			want:        true,
		},
		{
			name:      "not an uplink",
			portID:    "12",
			uplinkSet: map[string]struct{}{"49": {}, "50": {}},
			want:      false,
		},
		{
			name:        "AGGR no member is uplink",
			portID:      "AGGR/0",
			aggrMembers: []string{"1", "2"},
			uplinkSet:   map[string]struct{}{"49": {}},
			want:        false,
		},
		{
			name:      "empty uplink set",
			portID:    "12",
			uplinkSet: map[string]struct{}{},
			want:      false,
		},
		{
			name:        "nil AGGR members not uplink",
			portID:      "AGGR/1",
			aggrMembers: nil,
			uplinkSet:   map[string]struct{}{"49": {}},
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPortUplink(tt.portID, tt.aggrMembers, tt.uplinkSet)
			if got != tt.want {
				t.Errorf("isPortUplink(%q, %v, %v) = %v, want %v",
					tt.portID, tt.aggrMembers, tt.uplinkSet, got, tt.want)
			}
		})
	}
}

func TestResolveAggrPorts_NonAGGR(t *testing.T) {
	// Non-AGGR port: should return nil immediately (no client call needed).
	cache := map[string]map[string][]string{}
	result := resolveAggrPorts(context.TODO(), nil, "Q2HP-TEST", "12", cache)
	if result != nil {
		t.Errorf("resolveAggrPorts() non-AGGR = %v, want nil", result)
	}
}

func TestResolveAggrPorts_EmbeddedMembers(t *testing.T) {
	// AGGR port with embedded member list: should parse members without a client call.
	// The embedded path returns early, so no cache write occurs (and no client call needed).
	cache := map[string]map[string][]string{}
	result := resolveAggrPorts(context.TODO(), nil, "Q2HP-TEST",
		"AGGR/0=98:18:88:63:BA:37/49,98:18:88:63:BA:37/50",
		cache)
	if len(result) != 2 {
		t.Fatalf("resolveAggrPorts() embedded members len = %d, want 2", len(result))
	}
	if result[0] != "49" || result[1] != "50" {
		t.Errorf("resolveAggrPorts() embedded members = %v, want [49 50]", result)
	}
}

func TestResolveAggrPorts_CacheHit(t *testing.T) {
	// Pre-populate the cache; should return cached value without a client call.
	cache := map[string]map[string][]string{
		"Q2HP-TEST": {"AGGR/0": {"51", "52"}},
	}
	result := resolveAggrPorts(context.TODO(), nil, "Q2HP-TEST", "AGGR/0", cache)
	if len(result) != 2 || result[0] != "51" || result[1] != "52" {
		t.Errorf("resolveAggrPorts() cache hit = %v, want [51 52]", result)
	}
}
