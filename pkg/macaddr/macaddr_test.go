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

package macaddr

import (
	"testing"
)

// TestNormalizeExactMac checks the accepted MAC formats and the length and character errors.
func TestNormalizeExactMac(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "colon separated",
			input:   "00:11:22:33:44:55",
			want:    "001122334455",
			wantErr: false,
		},
		{
			name:    "no separators",
			input:   "001122334455",
			want:    "001122334455",
			wantErr: false,
		},
		{
			name:    "dot separated",
			input:   "00.11.22.33.44.55",
			want:    "001122334455",
			wantErr: false,
		},
		{
			name:    "mixed dot notation",
			input:   "0011.2233.4455",
			want:    "001122334455",
			wantErr: false,
		},
		{
			name:    "uppercase",
			input:   "AA:BB:CC:DD:EE:FF",
			want:    "aabbccddeeff",
			wantErr: false,
		},
		{
			name:    "invalid length",
			input:   "08:f1:b3",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "zz:f1:b3:6f:9c:25",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeExactMac(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeExactMac() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeExactMac() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFormatMacColon checks colon formatting of normalized MACs, including lowercasing.
func TestFormatMacColon(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "12 hex chars",
			input: "001122334455",
			want:  "00:11:22:33:44:55",
		},
		{
			name:  "uppercase",
			input: "AABBCCDDEEFF",
			want:  "aa:bb:cc:dd:ee:ff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMacColon(tt.input)
			if got != tt.want {
				t.Errorf("FormatMacColon() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildMacRegex checks that "*" matches one byte and bracket tokens match one nibble.
func TestBuildMacRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		matches []string
		noMatch []string
	}{
		{
			name:    "wildcard last byte",
			pattern: "0011223344*", // One * represents one byte (2 hex chars)
			matches: []string{"001122334400", "001122334455", "0011223344FF"},
			noMatch: []string{"001122334500", "021122334455"},
		},
		{
			name:    "bracket pattern",
			pattern: "0011223344[1-4][0-F]",
			matches: []string{"001122334410", "00112233442A", "00112233444F"},
			noMatch: []string{"001122334450", "00112233440A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := BuildMacRegex(tt.pattern)
			if err != nil {
				t.Fatalf("BuildMacRegex() error = %v", err)
			}
			for _, m := range tt.matches {
				if !re.MatchString(m) {
					t.Errorf("BuildMacRegex(%q) should match %q", tt.pattern, m)
				}
			}
			for _, m := range tt.noMatch {
				if re.MatchString(m) {
					t.Errorf("BuildMacRegex(%q) should not match %q", tt.pattern, m)
				}
			}
		})
	}
}

// TestBuildMacMatcher checks exact and wildcard matchers and the isPattern flag.
func TestBuildMacMatcher(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		testMACs    map[string]bool
		wantPattern bool
		wantErr     bool
	}{
		{
			name:  "exact MAC",
			input: "00:11:22:33:44:55",
			testMACs: map[string]bool{
				"001122334455": true,
				"001122334456": false,
			},
			wantPattern: false,
			wantErr:     false,
		},
		{
			name:  "wildcard pattern",
			input: "00:11:22:33:44:*", // User provides * for one byte
			testMACs: map[string]bool{
				"001122334400": true,
				"0011223344ff": true,
				"001122334555": false,
			},
			wantPattern: true,
			wantErr:     false,
		},
		{
			// The README example. The "-" inside a bracket is a range, not a
			// separator, so it must survive normalization.
			name:  "bracket range pattern",
			input: "08:f1:b3:6f:9c:[1-4][0-f]",
			testMACs: map[string]bool{
				"08f1b36f9c10": true,
				"08f1b36f9c2a": true,
				"08f1b36f9c4f": true,
				"08f1b36f9c50": false,
				"08f1b36f9c0a": false,
			},
			wantPattern: true,
			wantErr:     false,
		},
		{
			name:  "bracket range with dash separators",
			input: "08-f1-b3-6f-9c-[1-4][0-f]",
			testMACs: map[string]bool{
				"08f1b36f9c3c": true,
				"08f1b36f9c5c": false,
			},
			wantPattern: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, _, isPattern, err := BuildMacMatcher(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildMacMatcher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if isPattern != tt.wantPattern {
				t.Errorf("BuildMacMatcher() isPattern = %v, want %v", isPattern, tt.wantPattern)
			}
			if err == nil {
				for mac, shouldMatch := range tt.testMACs {
					if matcher(mac) != shouldMatch {
						t.Errorf("BuildMacMatcher(%q) matcher(%q) = %v, want %v", tt.input, mac, !shouldMatch, shouldMatch)
					}
				}
			}
		})
	}
}

// BenchmarkNormalizeExactMac measures normalizing a colon-separated MAC.
func BenchmarkNormalizeExactMac(b *testing.B) {
	mac := "00:11:22:33:44:55"
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeExactMac(mac)
	}
}

// BenchmarkBuildMacRegex measures compiling a bracket pattern.
func BenchmarkBuildMacRegex(b *testing.B) {
	pattern := "0011223344[1-4][0-F]"
	for i := 0; i < b.N; i++ {
		_, _ = BuildMacRegex(pattern)
	}
}
