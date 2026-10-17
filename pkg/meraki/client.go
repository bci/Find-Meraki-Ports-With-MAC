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

// Package meraki provides a client for the Cisco Meraki Dashboard API v1.
// It includes methods for querying organizations, networks, devices, clients,
// and live MAC table lookups with automatic pagination and retry logic.
package meraki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Organization represents a Meraki organization as returned by
// GET /organizations. Only the fields this tool uses are decoded.
type Organization struct {
	// ID is the Meraki organization ID used in API paths.
	ID string `json:"id"`
	// Name is the human-readable organization name shown in the Dashboard.
	Name string `json:"name"`
}

// Network represents a Meraki network as returned by
// GET /organizations/{organizationId}/networks.
type Network struct {
	// ID is the Meraki network ID (e.g. "L_123..." or "N_123...").
	ID string `json:"id"`
	// Name is the human-readable network name shown in the Dashboard.
	Name string `json:"name"`
}

// Device represents a Meraki device (switch, access point, etc.) as returned
// by GET /networks/{networkId}/devices.
type Device struct {
	// Serial is the device serial number, the key for every /devices/{serial} call.
	Serial string `json:"serial"`
	// Name is the user-assigned device name; may be empty.
	Name string `json:"name"`
	// Model is the hardware model, e.g. "MS225-48FP" or "C9300-48P".
	Model string `json:"model"`
	// ProductType is the Meraki product family, e.g. "switch" or "wireless".
	ProductType string `json:"productType"`
	// NetworkID is the ID of the network the device belongs to.
	NetworkID string `json:"networkId"`
}

// Client represents a client connected to a device, as returned by
// GET /devices/{serial}/clients.
//
// The port can arrive in any of three fields depending on device model and
// firmware, so callers try SwitchportName, Switchport and Port in that order.
type Client struct {
	// MAC is the client MAC address in whatever format the API returns.
	MAC string `json:"mac"`
	// Switchport is the switch port the client was seen on.
	Switchport string `json:"switchport"`
	// SwitchportName is the port's display name, preferred when present.
	SwitchportName string `json:"switchportName"`
	// Port is an alternate port field returned by some models.
	Port string `json:"port"`
	// LastSeen is the time the client was last seen, as returned by the API.
	LastSeen string `json:"lastSeen"`
}

// NetworkClient represents a client at the network level, as returned by
// GET /networks/{networkId}/clients. It is the source of IP-to-MAC mappings
// for --ip lookups and of hostname hints (see ClientHostname).
type NetworkClient struct {
	// MAC is the client MAC address.
	MAC string `json:"mac"`
	// Switchport is the switch port the client was last seen on.
	Switchport string `json:"switchport"`
	// SwitchportName is the port's display name, preferred when present.
	SwitchportName string `json:"switchportName"`
	// Port is an alternate port field returned by some models.
	Port string `json:"port"`
	// LastSeen is the time the client was last seen, as returned by the API.
	LastSeen string `json:"lastSeen"`
	// RecentDeviceSerial is the serial of the device the client was last seen on.
	RecentDeviceSerial string `json:"recentDeviceSerial"`
	// RecentDeviceName is the name of the device the client was last seen on.
	RecentDeviceName string `json:"recentDeviceName"`
	// IP is the client's most recent IPv4 address.
	IP string `json:"ip"`
	// Hostname is the hostname Meraki reports for the client, if any.
	Hostname string `json:"hostname"`
	// Description is the client description; Meraki sometimes fills it with a UUID.
	Description string `json:"description"`
	// DhcpHostname is the hostname the client sent in its DHCP request.
	DhcpHostname string `json:"dhcpHostname"`
	// Notes is the free-text note an administrator attached to the client.
	Notes string `json:"notes"`
}

// MerakiClient is an HTTP client wrapper for the Meraki Dashboard API.
// Create one with NewClient. It is safe for sequential use; it holds no
// per-request state.
type MerakiClient struct {
	// apiKey is sent as the X-Cisco-Meraki-API-Key header on every request.
	apiKey string
	// baseURL is the API root without a trailing slash.
	baseURL string
	// maxRetries is the number of attempts doRequest makes on HTTP 429.
	maxRetries int
	// client is the underlying HTTP client (60-second timeout).
	client *http.Client
}

// NewClient creates a new Meraki API client.
//
// apiKey is the Dashboard API key. baseURL is the API root; "" uses
// https://api.meraki.com/api/v1 and a trailing slash is trimmed. maxRetries
// controls how many times a 429 response is retried; 0 or less uses the
// default of 6. Each HTTP request has a 60-second timeout.
func NewClient(apiKey, baseURL string, maxRetries int) *MerakiClient {
	if baseURL == "" {
		baseURL = "https://api.meraki.com/api/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if maxRetries <= 0 {
		maxRetries = 6
	}
	return &MerakiClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		maxRetries: maxRetries,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetOrganizations retrieves all organizations accessible by the API key,
// following pagination. Entries that fail to decode are skipped silently.
// It returns an error if any page request fails.
func (m *MerakiClient) GetOrganizations(ctx context.Context) ([]Organization, error) {
	raws, err := m.getAllPages(ctx, "/organizations", url.Values{"perPage": []string{"1000"}})
	if err != nil {
		return nil, err
	}
	orgs := make([]Organization, 0, len(raws))
	for _, r := range raws {
		var o Organization
		if err := json.Unmarshal(r, &o); err == nil {
			orgs = append(orgs, o)
		}
	}
	return orgs, nil
}

// GetNetworks retrieves all networks for the organization orgID, following
// pagination. Entries that fail to decode are skipped. It returns an error if
// any page request fails.
func (m *MerakiClient) GetNetworks(ctx context.Context, orgID string) ([]Network, error) {
	path := fmt.Sprintf("/organizations/%s/networks", orgID)
	raws, err := m.getAllPages(ctx, path, url.Values{"perPage": []string{"1000"}})
	if err != nil {
		return nil, err
	}
	nets := make([]Network, 0, len(raws))
	for _, r := range raws {
		var n Network
		if err := json.Unmarshal(r, &n); err == nil {
			nets = append(nets, n)
		}
	}
	return nets, nil
}

// GetDevices retrieves all devices in the network networkID, following
// pagination. Entries that fail to decode are skipped. It returns an error if
// any page request fails.
func (m *MerakiClient) GetDevices(ctx context.Context, networkID string) ([]Device, error) {
	path := fmt.Sprintf("/networks/%s/devices", networkID)
	raws, err := m.getAllPages(ctx, path, url.Values{"perPage": []string{"1000"}})
	if err != nil {
		return nil, err
	}
	devs := make([]Device, 0, len(raws))
	for _, r := range raws {
		var d Device
		if err := json.Unmarshal(r, &d); err == nil {
			devs = append(devs, d)
		}
	}
	return devs, nil
}

// GetDeviceClients retrieves clients connected to the device with the given
// serial. It uses a 30-day timespan so that recently idle devices are still
// found. Entries that fail to decode are skipped; a failed request returns an
// error.
func (m *MerakiClient) GetDeviceClients(ctx context.Context, serial string) ([]Client, error) {
	path := fmt.Sprintf("/devices/%s/clients", serial)
	params := url.Values{
		"perPage":  []string{"1000"},
		"timespan": []string{"2592000"}, // 30 days
	}
	raws, err := m.getAllPages(ctx, path, params)
	if err != nil {
		return nil, err
	}
	clients := make([]Client, 0, len(raws))
	for _, r := range raws {
		var c Client
		if err := json.Unmarshal(r, &c); err == nil {
			clients = append(clients, c)
		}
	}
	return clients, nil
}

// GetNetworkClients retrieves all clients across the network networkID. It
// uses a 30-day timespan so that recently idle devices are still found.
// Entries that fail to decode are skipped; a failed request returns an error.
func (m *MerakiClient) GetNetworkClients(ctx context.Context, networkID string) ([]NetworkClient, error) {
	path := fmt.Sprintf("/networks/%s/clients", networkID)
	params := url.Values{
		"perPage":  []string{"1000"},
		"timespan": []string{"2592000"}, // 30 days
	}
	raws, err := m.getAllPages(ctx, path, params)
	if err != nil {
		return nil, err
	}
	clients := make([]NetworkClient, 0, len(raws))
	for _, r := range raws {
		var c NetworkClient
		if err := json.Unmarshal(r, &c); err == nil {
			clients = append(clients, c)
		}
	}
	return clients, nil
}

// CreateMacTableLookup initiates a live MAC table lookup on the device with
// the given serial (POST /devices/{serial}/liveTools/macTable).
//
// This is critical for Cisco Catalyst switches managed by Meraki, whose
// client endpoints often have limited visibility. It returns the macTableId
// to pass to GetMacTableLookup, or an error if the request fails or the
// response carries no macTableId.
func (m *MerakiClient) CreateMacTableLookup(ctx context.Context, serial string) (string, error) {
	path := fmt.Sprintf("/devices/%s/liveTools/macTable", serial)
	body, _, err := m.doRequest(ctx, "POST", m.buildURL(path, nil))
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	macTableID, ok := result["macTableId"].(string)
	if !ok {
		return "", fmt.Errorf("no macTableId in response")
	}
	return macTableID, nil
}

// GetMacTableLookup polls for the results of a live MAC table lookup started
// by CreateMacTableLookup.
//
// Returns:
//   - entries: array of MAC table entries (when status is "complete")
//   - status: "pending", "complete", or "failed"
//   - error: any errors during the request
func (m *MerakiClient) GetMacTableLookup(ctx context.Context, serial, macTableID string) ([]map[string]interface{}, string, error) {
	path := fmt.Sprintf("/devices/%s/liveTools/macTable/%s", serial, macTableID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return nil, "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}

	status, _ := result["status"].(string)
	if status != "complete" {
		return nil, status, nil
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		return nil, status, nil
	}

	var macEntries []map[string]interface{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			macEntries = append(macEntries, entry)
		}
	}

	return macEntries, status, nil
}

// CreateArpTableLookup initiates a live ARP table lookup on the device with
// the given serial (POST /devices/{serial}/liveTools/arpTable). It returns the
// arpTableId to pass to GetArpTableLookup, or an error if the request fails
// or the response carries no arpTableId.
func (m *MerakiClient) CreateArpTableLookup(ctx context.Context, serial string) (string, error) {
	path := fmt.Sprintf("/devices/%s/liveTools/arpTable", serial)
	body, _, err := m.doRequest(ctx, "POST", m.buildURL(path, nil))
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	id, ok := result["arpTableId"].(string)
	if !ok {
		return "", fmt.Errorf("no arpTableId in response")
	}
	return id, nil
}

// GetArpTableLookup polls for the results of a live ARP table lookup started
// by CreateArpTableLookup. It returns the entries (each a map with "ip" and
// "mac" keys) once status is "complete"; for any other status the entries are
// nil. err is non-nil only when the request or JSON decoding fails.
func (m *MerakiClient) GetArpTableLookup(ctx context.Context, serial, arpTableID string) ([]map[string]interface{}, string, error) {
	path := fmt.Sprintf("/devices/%s/liveTools/arpTable/%s", serial, arpTableID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return nil, "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}
	status, _ := result["status"].(string)
	if status != "complete" {
		return nil, status, nil
	}
	entries, ok := result["entries"].([]interface{})
	if !ok {
		return nil, status, nil
	}
	var out []map[string]interface{}
	for _, e := range entries {
		if entry, ok := e.(map[string]interface{}); ok {
			out = append(out, entry)
		}
	}
	return out, status, nil
}

// FetchArpMap creates and polls a live ARP table for a device, returning a
// normalized-MAC → IP map. maxPoll is the number of 2-second poll attempts.
// Returns an empty map (not an error) when the device doesn't support ARP table.
func (m *MerakiClient) FetchArpMap(ctx context.Context, serial string, maxPoll int) map[string]string {
	result := make(map[string]string)
	arpID, err := m.CreateArpTableLookup(ctx, serial)
	if err != nil {
		return result
	}
	for i := 0; i < maxPoll; i++ {
		time.Sleep(2 * time.Second)
		entries, status, err := m.GetArpTableLookup(ctx, serial, arpID)
		if err != nil || status == "failed" {
			return result
		}
		if status == "complete" {
			for _, e := range entries {
				ip, _ := e["ip"].(string)
				mac, _ := e["mac"].(string)
				if ip == "" || mac == "" {
					continue
				}
				// normalize MAC (strip separators, lowercase)
				clean := strings.Map(func(r rune) rune {
					if r == ':' || r == '.' || r == '-' {
						return -1
					}
					return r
				}, strings.ToLower(mac))
				result[clean] = ip
			}
			return result
		}
	}
	return result
}

// getAllPages handles pagination for API endpoints that return arrays.
// It follows the Link header with rel="next" until all pages are retrieved,
// and returns every array element as raw JSON so callers can decode into
// their own types. Any failed page aborts the whole call with an error.
func (m *MerakiClient) getAllPages(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error) {
	fullURL := m.buildURL(path, params)
	var all []json.RawMessage
	for {
		body, next, err := m.doRequest(ctx, "GET", fullURL)
		if err != nil {
			return nil, err
		}
		var page []json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		fullURL = next
	}
	return all, nil
}

// buildURL constructs a full API URL from a path (a leading "/" is added if
// missing) and optional query parameters.
func (m *MerakiClient) buildURL(path string, params url.Values) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base := m.baseURL + path
	if len(params) == 0 {
		return base
	}
	return base + "?" + params.Encode()
}

// doRequest executes an HTTP request with retry logic and rate limit handling.
// It automatically retries on 429 (Too Many Requests), sleeping for the
// Retry-After header when present and otherwise for 1s, 2s, 3s, ... (linear
// backoff), up to maxRetries attempts. Transport errors are not retried.
// Returns the response body, next page URL (from Link header), and any error;
// a status of 300 or above is returned as an error containing the body.
func (m *MerakiClient) doRequest(ctx context.Context, method, fullURL string) ([]byte, string, error) {
	for attempt := 0; attempt < m.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("X-Cisco-Meraki-API-Key", m.apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := m.client.Do(req)
		if err != nil {
			return nil, "", err
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
					time.Sleep(seconds)
					continue
				}
			}
			time.Sleep(time.Second * time.Duration(1+attempt))
			continue
		}

		if resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("meraki API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		next := parseLinkNext(resp.Header.Get("Link"))
		return body, next, nil
	}
	return nil, "", errors.New("meraki API request failed after retries")
}

// customDNSServers holds optional user-supplied DNS server addresses (host:port).
// Set via SetDNSServers before calling ResolveHostname.
var customDNSServers []string

// hostOverrides is a scoped IP→hostname map, keyed by "orgName/netName".
// Use "*" as a wildcard for either part. Set via SetHostOverrides.
var hostOverrides map[string]map[string]string

// hostOverrideEntry is the JSON shape for a single HOST_OVERRIDES entry.
type hostOverrideEntry struct {
	// Org is the organization name the entry applies to, or "*" / "" for any.
	Org string `json:"org"`
	// Net is the network name the entry applies to, or "*" / "" for any.
	Net string `json:"net"`
	// IP is the address being mapped.
	IP string `json:"ip"`
	// Hostname is the name to report for IP.
	Hostname string `json:"hostname"`
}

// SetHostOverrides installs a scoped static IP-to-hostname map for sites where
// the internal DNS server is not reachable from the machine running this tool.
// It replaces any previously installed map. Invalid JSON, or a list with no
// usable entries, clears the map silently (no error is reported).
//
// Format: JSON array of objects with "org", "net", "ip", and "hostname" fields.
// Use "*" as a wildcard for "org" or "net". Omitted "org"/"net" fields default to "*".
//
// Example:
//
//	[{"org":"Acme Corp","net":"HQ","ip":"192.168.1.1","hostname":"gateway"},{"org":"*","net":"*","ip":"10.0.0.1","hostname":"core"}]
func SetHostOverrides(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		hostOverrides = nil
		return
	}
	var entries []hostOverrideEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		hostOverrides = nil
		return
	}
	m := make(map[string]map[string]string)
	for _, e := range entries {
		org := strings.TrimSpace(e.Org)
		net := strings.TrimSpace(e.Net)
		ip := strings.TrimSpace(e.IP)
		hn := strings.TrimSpace(e.Hostname)
		if ip == "" || hn == "" {
			continue
		}
		if org == "" {
			org = "*"
		}
		if net == "" {
			net = "*"
		}
		scope := org + "/" + net
		if _, ok := m[scope]; !ok {
			m[scope] = make(map[string]string)
		}
		m[scope][ip] = hn
	}
	if len(m) > 0 {
		hostOverrides = m
	} else {
		hostOverrides = nil
	}
}

// LookupHostOverride returns the static hostname override for the given IP
// within the specified org and network. Lookup priority:
//  1. orgName/netName  (exact match)
//  2. orgName/*        (org matches, any network)
//  3. */netName        (any org, network matches)
//  4. */*              (global fallback / backward-compat bare entries)
//
// Returns "" if no override is found.
func LookupHostOverride(ip, orgName, netName string) string {
	if hostOverrides == nil {
		return ""
	}
	for _, key := range []string{
		orgName + "/" + netName,
		orgName + "/*",
		"*/" + netName,
		"*/*",
	} {
		if m, ok := hostOverrides[key]; ok {
			if hn, ok := m[ip]; ok {
				return hn
			}
		}
	}
	return ""
}

// SetDNSServers configures one or more DNS servers for reverse hostname lookups.
// Each entry should be "host" or "host:port"; bare IPs get ":53" appended.
// Pass nil or an empty slice to revert to the system default resolver.
func SetDNSServers(servers []string) {
	cleaned := make([]string, 0, len(servers))
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, ":") {
			s += ":53"
		}
		cleaned = append(cleaned, s)
	}
	customDNSServers = cleaned
}

// ResolveHostname performs a reverse DNS (PTR) lookup on an IP address, with
// a 5-second overall timeout, using the servers set by SetDNSServers or the
// system resolver when none are set. It returns the first name with its
// trailing dot removed, or "" and the lookup error when it fails. An empty ip
// returns "", nil.
func ResolveHostname(ip string) (string, error) {
	if ip == "" {
		return "", nil
	}

	// Use a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resolver := net.DefaultResolver
	if len(customDNSServers) > 0 {
		servers := customDNSServers // capture for closure
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				// Try each configured server, return on first success.
				var lastErr error
				for _, srv := range servers {
					conn, err := d.DialContext(ctx, "udp", srv)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, lastErr
			},
		}
	}

	// Perform reverse DNS lookup
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return "", err
	}

	// Return the first name, trim trailing dot
	return strings.TrimSuffix(names[0], "."), nil
}

// isUUIDLike returns true if s matches the xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx pattern.
// Used to filter out auto-generated UUIDs that Meraki sometimes stores in client Description.
func isUUIDLike(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// ClientHostname returns the best available hostname for a NetworkClient.
// Priority: Notes > Hostname (API field) > DhcpHostname > Description (if not UUID-like).
// Notes wins because it is what an administrator typed by hand. Returns "" if
// none is set.
func ClientHostname(nc NetworkClient) string {
	if nc.Notes != "" {
		return nc.Notes
	}
	if nc.Hostname != "" {
		return nc.Hostname
	}
	if nc.DhcpHostname != "" {
		return nc.DhcpHostname
	}
	if nc.Description != "" && !isUUIDLike(nc.Description) {
		return nc.Description
	}
	return ""
}

// SwitchPort represents the configuration of a Meraki switch port, as
// returned by GET /devices/{serial}/switch/ports/{portId}.
type SwitchPort struct {
	// Number may be int or string depending on switch model.
	Number interface{} `json:"number"` // may be int or string depending on switch model
	// Name is the port's configured name.
	Name string `json:"name"`
	// Type is the port mode: "access" or "trunk".
	Type string `json:"type"` // "access" or "trunk"
	// Vlan is the access VLAN (access ports) or native VLAN (trunk ports).
	Vlan int `json:"vlan"` // access VLAN (access ports)
	// VoiceVlan is the voice VLAN; decoded but not used by this tool.
	VoiceVlan int `json:"voiceVlan"` // voice VLAN (ignored here)
}

// GetSwitchPort retrieves the configuration for a single switch port.
// portID is the port number/name as a string (e.g. "24", "1"). It returns an
// error if the request fails (for example, for virtual AGGR ports, which have
// no entry) or the body cannot be decoded.
func (m *MerakiClient) GetSwitchPort(ctx context.Context, serial, portID string) (*SwitchPort, error) {
	path := fmt.Sprintf("/devices/%s/switch/ports/%s", serial, portID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return nil, err
	}
	var sp SwitchPort
	if err := json.Unmarshal(body, &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

// SwitchPortFull holds the full port detail needed to resolve link-aggregation
// membership. It is currently unused: GetSwitchPortMembers decodes raw maps
// instead, so that alternate field names can be tried.
type SwitchPortFull struct {
	// PortID is the switch port ID.
	PortID string `json:"portId"`
	// LinkAggregationID is the LAG the port belongs to, if any.
	LinkAggregationID string `json:"linkAggregationId"` // e.g. "AGGR/1" when port is a LAG member
}

// GetSwitchPortMembers returns a map of aggregation-port-ID → sorted list of member port IDs
// for the given switch, e.g. {"AGGR/1": ["1","2"], "AGGR/2": ["3","4"]}.
// Returns an empty map (never nil) on error so callers can safely do a lookup.
func (m *MerakiClient) GetSwitchPortMembers(ctx context.Context, serial string) map[string][]string {
	path := fmt.Sprintf("/devices/%s/switch/ports", serial)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return map[string][]string{}
	}
	// Unmarshal as raw maps so we can inspect all fields for debugging
	var rawPorts []map[string]interface{}
	if err := json.Unmarshal(body, &rawPorts); err != nil {
		return map[string][]string{}
	}
	aggr := make(map[string][]string)
	for _, rp := range rawPorts {
		portID, _ := rp["portId"].(string)
		lagID, _ := rp["linkAggregationId"].(string)
		if lagID == "" {
			// Also try alternate field names Meraki might use
			lagID, _ = rp["lagId"].(string)
		}
		if lagID == "" || portID == "" {
			continue
		}
		aggr[lagID] = append(aggr[lagID], portID)
	}
	// Sort member port lists for stable output
	for k := range aggr {
		sort.Slice(aggr[k], func(i, j int) bool {
			ai, ei := strconv.Atoi(aggr[k][i])
			aj, ej := strconv.Atoi(aggr[k][j])
			if ei == nil && ej == nil {
				return ai < aj
			}
			return aggr[k][i] < aggr[k][j]
		})
	}
	return aggr
}

// TopologyNode represents a device node in the network link-layer topology.
type TopologyNode struct {
	// MAC is the device MAC address.
	MAC string `json:"mac"`
	// Type is the device type reported by the topology API.
	Type string `json:"type"`
	// Name is the device name.
	Name string `json:"name"`
	// Serial is the device serial; empty for devices Meraki does not manage.
	Serial string `json:"serial"`
	// DerivedRole is the role Meraki infers for the device in the topology.
	DerivedRole string `json:"derivedRole"`
}

// TopologyEnd represents one end of a topology link.
type TopologyEnd struct {
	// Device is the device at this end of the link.
	Device TopologyNode `json:"device"`
	// IpAddr is the device's IP address on this link, if reported.
	IpAddr string `json:"ipAddr"`
	// PortId is the switch port ID on this side of the link. On some firmware
	// it is not populated.
	PortId string `json:"portId"` // switch port ID on this side of the link
}

// TopologyLink represents a connection between two devices.
type TopologyLink struct {
	// LastUpdatedAt is when Meraki last observed the link.
	LastUpdatedAt string `json:"lastUpdatedAt"`
	// Ends holds the two ends of the link (normally exactly two).
	Ends []TopologyEnd `json:"ends"`
}

// BuildUplinkPortSet returns a map of serial → set of portIDs that are
// inter-device (uplink) ports, derived from the link-layer topology.
// Only ends that connect two switches (both ends have a serial) are included.
// A nil topo yields an empty map. It is not called by the current lookup
// paths, which use GetDeviceUplinkPorts instead.
func BuildUplinkPortSet(topo *TopologyData) map[string]map[string]struct{} {
	uplinks := make(map[string]map[string]struct{})
	if topo == nil {
		return uplinks
	}
	add := func(serial, portID string) {
		if serial == "" || portID == "" {
			return
		}
		if uplinks[serial] == nil {
			uplinks[serial] = make(map[string]struct{})
		}
		uplinks[serial][portID] = struct{}{}
	}
	for _, link := range topo.Links {
		if len(link.Ends) < 2 {
			continue
		}
		// Both ends must reference a device with a serial (i.e. a managed switch/AP).
		if link.Ends[0].Device.Serial == "" || link.Ends[1].Device.Serial == "" {
			continue
		}
		add(link.Ends[0].Device.Serial, link.Ends[0].PortId)
		add(link.Ends[1].Device.Serial, link.Ends[1].PortId)
	}
	return uplinks
}

// TopologyData holds nodes and links for a network's link-layer topology.
type TopologyData struct {
	// Nodes are kept as raw maps because their shape varies by device type.
	Nodes []map[string]interface{} `json:"nodes"`
	// Links are the device-to-device connections.
	Links []TopologyLink `json:"links"`
}

// GetNetworkTopology retrieves the link-layer topology for a network
// (GET /networks/{networkId}/topology/linkLayer). It returns an error if the
// request fails or the body cannot be decoded.
func (m *MerakiClient) GetNetworkTopology(ctx context.Context, networkID string) (*TopologyData, error) {
	path := fmt.Sprintf("/networks/%s/topology/linkLayer", networkID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return nil, err
	}
	var result TopologyData
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNetworkTopologyRaw retrieves the raw JSON bytes for the link-layer
// topology of a network. It is used by the /api/debug/network handler.
func (m *MerakiClient) GetNetworkTopologyRaw(ctx context.Context, networkID string) ([]byte, error) {
	path := fmt.Sprintf("/networks/%s/topology/linkLayer", networkID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	return body, err
}

// GetSwitchPortsRaw retrieves the raw JSON bytes for all ports on a switch.
// It is used by the /api/debug/network handler.
func (m *MerakiClient) GetSwitchPortsRaw(ctx context.Context, serial string) ([]byte, error) {
	path := fmt.Sprintf("/devices/%s/switch/ports", serial)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	return body, err
}

// NetworkLinkAggregation represents a network-level link aggregation group,
// as returned by GET /networks/{networkId}/switch/linkAggregations.
type NetworkLinkAggregation struct {
	// ID is the Meraki link aggregation ID.
	ID string `json:"id"`
	// SwitchPorts lists the member ports, each identified by switch serial and port ID.
	SwitchPorts []struct {
		Serial string `json:"serial"`
		PortID string `json:"portId"`
	} `json:"switchPorts"`
}

// GetNetworkLinkAggregations returns a map of serial → (aggrIndex → []portIDs)
// built from the network-level /networks/{id}/switch/linkAggregations endpoint.
// The aggrIndex is 0-based, assigned in the order the LAGs appear for that serial,
// matching the "AGGR/0", "AGGR/1"... IDs that Meraki uses in the MAC table.
// Returns an empty map on error.
func (m *MerakiClient) GetNetworkLinkAggregations(ctx context.Context, networkID string) map[string]map[string][]string {
	path := fmt.Sprintf("/networks/%s/switch/linkAggregations", networkID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return map[string]map[string][]string{}
	}
	var lags []NetworkLinkAggregation
	if err := json.Unmarshal(body, &lags); err != nil {
		return map[string]map[string][]string{}
	}

	// Group LAGs by switch serial so we can assign AGGR/0, AGGR/1, etc. per switch.
	// We track the order each LAG index is assigned per serial to match MAC table naming.
	result := make(map[string]map[string][]string)
	// For each LAG, collect the port IDs grouped by serial
	for _, lag := range lags {
		// Build serial→portIDs for this LAG
		bySerial := make(map[string][]string)
		for _, sp := range lag.SwitchPorts {
			if sp.Serial == "" || sp.PortID == "" {
				continue
			}
			bySerial[sp.Serial] = append(bySerial[sp.Serial], sp.PortID)
		}
		for serial, portIDs := range bySerial {
			if result[serial] == nil {
				result[serial] = make(map[string][]string)
			}
			// Assign next available AGGR index for this serial
			idx := 0
			for {
				key := fmt.Sprintf("AGGR/%d", idx)
				if _, exists := result[serial][key]; !exists {
					// Sort ports numerically for stable output
					sort.Slice(portIDs, func(i, j int) bool {
						ai, ei := strconv.Atoi(portIDs[i])
						aj, ej := strconv.Atoi(portIDs[j])
						if ei == nil && ej == nil {
							return ai < aj
						}
						return portIDs[i] < portIDs[j]
					})
					result[serial][key] = portIDs
					break
				}
				idx++
			}
		}
	}
	return result
}

// LLDPCDPData holds the LLDP/CDP neighbor data for a device. It is currently
// unused: uplink detection moved to the switch port statuses API (see
// GetDeviceUplinkPorts).
type LLDPCDPData struct {
	// Ports maps port ID string → map of protocol ("lldp"/"cdp") → neighbor info
	Ports map[string]map[string]interface{} `json:"ports"`
	// SourceMac is the MAC address of the device reporting the neighbors.
	SourceMac string `json:"sourceMac"`
}

// GetDeviceUplinkPorts returns the set of port IDs on the given switch that are
// marked as uplinks by the Meraki platform, using the switch port statuses API
// (/devices/{serial}/switch/ports/statuses). This mirrors exactly what the
// Meraki console shows as "uplink" — including ports connected to ISP routers,
// core switches, and other upstream devices regardless of whether they speak
// LLDP or CDP.
// Returns an empty set (never nil) on error.
func (m *MerakiClient) GetDeviceUplinkPorts(ctx context.Context, serial string) map[string]struct{} {
	path := fmt.Sprintf("/devices/%s/switch/ports/statuses", serial)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return map[string]struct{}{}
	}
	var ports []struct {
		PortID   string `json:"portId"`
		IsUplink bool   `json:"isUplink"`
	}
	if err := json.Unmarshal(body, &ports); err != nil {
		return map[string]struct{}{}
	}
	uplinks := make(map[string]struct{})
	for _, p := range ports {
		if p.IsUplink {
			uplinks[p.PortID] = struct{}{}
		}
	}
	return uplinks
}

// ResolveIPToMAC resolves an IP address to MAC address by querying Meraki clients API.
// Searches across multiple networks and returns the MAC, network ID, and hostname.
//
// The hostname comes from a reverse DNS lookup first and, if that is empty,
// from ClientHostname of the matching client. Networks whose client list
// cannot be fetched are skipped silently. orgID is currently unused. If no
// network has a client with exactly this IP, it returns an error (with the
// DNS hostname, if any).
func (c *MerakiClient) ResolveIPToMAC(ctx context.Context, orgID string, networks []Network, ip string) (mac string, networkID string, hostname string, err error) {
	// First, attempt hostname resolution
	hostname, _ = ResolveHostname(ip) // Ignore error, hostname is optional

	// Search through each network for the IP
	for _, network := range networks {
		clients, err := c.GetNetworkClients(ctx, network.ID)
		if err != nil {
			continue // Skip network on error
		}

		for _, client := range clients {
			if client.IP == ip {
				if hostname == "" {
					hostname = ClientHostname(client)
				}
				return client.MAC, network.ID, hostname, nil
			}
		}
	}

	return "", "", hostname, errors.New("IP address not found in any network")
}

// parseLinkNext extracts the next page URL from a Link header, or returns ""
// when there is no rel="next" entry.
// Example Link header: <https://api.meraki.com/api/v1/...?page=2>; rel="next"
func parseLinkNext(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		section := strings.TrimSpace(part)
		if !strings.Contains(section, "rel=\"next\"") {
			continue
		}
		start := strings.Index(section, "<")
		end := strings.Index(section, ">")
		if start == -1 || end == -1 || end <= start+1 {
			continue
		}
		return section[start+1 : end]
	}
	return ""
}
