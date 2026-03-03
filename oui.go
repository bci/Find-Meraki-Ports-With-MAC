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
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ouiCache stores OUI prefix → vendor name to avoid duplicate API calls.
var ouiCache sync.Map

// lookupOUI queries api.macvendors.com for the vendor of a MAC address.
// The first three octets (OUI) are used as the cache key.
// Returns empty string if the lookup fails or the vendor is unknown.
func lookupOUI(mac string) string {
	if mac == "" {
		return ""
	}
	// Normalise separators and extract the OUI prefix (first 8 chars: XX:XX:XX)
	norm := strings.ToUpper(strings.NewReplacer("-", ":", ".", ":").Replace(mac))
	parts := strings.Split(norm, ":")
	if len(parts) < 3 {
		return ""
	}
	oui := strings.Join(parts[:3], ":")

	if cached, ok := ouiCache.Load(oui); ok {
		return cached.(string)
	}

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.macvendors.com/" + oui)
	if err != nil {
		ouiCache.Store(oui, "")
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		ouiCache.Store(oui, "")
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		ouiCache.Store(oui, "")
		return ""
	}
	vendor := strings.TrimSpace(string(body))
	ouiCache.Store(oui, vendor)
	return vendor
}

func getManufacturer(mac string) string {
	return lookupOUI(mac)
}
