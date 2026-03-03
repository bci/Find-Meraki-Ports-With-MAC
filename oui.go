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
