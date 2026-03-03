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

// Organization represents a Meraki organization.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Network represents a Meraki network.
type Network struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Device represents a Meraki device (switch, access point, etc.).
type Device struct {
	Serial      string `json:"serial"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	ProductType string `json:"productType"`
	NetworkID   string `json:"networkId"`
}

// Client represents a client connected to a device.
type Client struct {
	MAC            string `json:"mac"`
	Switchport     string `json:"switchport"`
	SwitchportName string `json:"switchportName"`
	Port           string `json:"port"`
	LastSeen       string `json:"lastSeen"`
}

// NetworkClient represents a client at the network level.
type NetworkClient struct {
	MAC                string `json:"mac"`
	Switchport         string `json:"switchport"`
	SwitchportName     string `json:"switchportName"`
	Port               string `json:"port"`
	LastSeen           string `json:"lastSeen"`
	RecentDeviceSerial string `json:"recentDeviceSerial"`
	RecentDeviceName   string `json:"recentDeviceName"`
	IP                 string `json:"ip"`
	Hostname           string `json:"hostname"`
	Description        string `json:"description"`
	DhcpHostname       string `json:"dhcpHostname"`
	Notes              string `json:"notes"`
}

// MerakiClient is an HTTP client wrapper for the Meraki Dashboard API.
type MerakiClient struct {
	apiKey     string
	baseURL    string
	maxRetries int
	client     *http.Client
}

// NewClient creates a new Meraki API client.
// maxRetries controls how many times a 429 response is retried; 0 uses the default of 6.
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

// GetOrganizations retrieves all organizations accessible by the API key.
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

// GetNetworks retrieves all networks for a given organization.
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

// GetDevices retrieves all devices in a network.
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

// GetDeviceClients retrieves clients connected to a specific device.
// Uses a 30-day timespan for historical data.
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

// GetNetworkClients retrieves all clients across a network.
// Uses a 30-day timespan for historical data.
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

// CreateMacTableLookup initiates a live MAC table lookup on a device.
// Returns the macTableId which can be used to poll for results.
// This is critical for Cisco Catalyst switches managed by Meraki.
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

// GetMacTableLookup polls for the results of a live MAC table lookup.
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

// CreateArpTableLookup initiates a live ARP table lookup on a device.
// Returns the arpTableId which can be used to poll for results.
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

// GetArpTableLookup polls for the results of a live ARP table lookup.
// Returns entries (map of "ip"→"mac"), status, and any error.
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
// It follows the Link header with rel="next" until all pages are retrieved.
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

// buildURL constructs a full API URL from a path and query parameters.
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
// It automatically retries on 429 (Too Many Requests) with exponential backoff.
// Returns the response body, next page URL (from Link header), and any error.
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
	Org      string `json:"org"`
	Net      string `json:"net"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// SetHostOverrides installs a scoped static IP-to-hostname map for sites where
// the internal DNS server is not reachable from the machine running this tool.
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

// ResolveHostname performs reverse DNS lookup on an IP address.
// Returns the hostname or empty string if lookup fails.
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
// Priority: Notes > Hostname (API field) > DhcpHostname > Description (if not UUID-like)
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

// SwitchPort represents the configuration of a Meraki switch port.
type SwitchPort struct {
	Number    interface{} `json:"number"` // may be int or string depending on switch model
	Name      string      `json:"name"`
	Type      string      `json:"type"`      // "access" or "trunk"
	Vlan      int         `json:"vlan"`      // access VLAN (access ports)
	VoiceVlan int         `json:"voiceVlan"` // voice VLAN (ignored here)
}

// GetSwitchPort retrieves the configuration for a single switch port.
// portID is the port number/name as a string (e.g. "24", "1").
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

// SwitchPortFull holds the full port detail needed to resolve link-aggregation membership.
type SwitchPortFull struct {
	PortID            string `json:"portId"`
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
	MAC         string `json:"mac"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Serial      string `json:"serial"`
	DerivedRole string `json:"derivedRole"`
}

// TopologyEnd represents one end of a topology link.
type TopologyEnd struct {
	Device TopologyNode `json:"device"`
	IpAddr string       `json:"ipAddr"`
	PortId string       `json:"portId"` // switch port ID on this side of the link
}

// TopologyLink represents a connection between two devices.
type TopologyLink struct {
	LastUpdatedAt string        `json:"lastUpdatedAt"`
	Ends          []TopologyEnd `json:"ends"`
}

// BuildUplinkPortSet returns a map of serial → set of portIDs that are
// inter-device (uplink) ports, derived from the link-layer topology.
// Only ends that connect two switches (both ends have a serial) are included.
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
	Nodes []map[string]interface{} `json:"nodes"`
	Links []TopologyLink           `json:"links"`
}

// GetNetworkTopology retrieves the link-layer topology for a network.
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

// GetNetworkTopologyRaw retrieves the raw JSON bytes for the link-layer topology of a network.
func (m *MerakiClient) GetNetworkTopologyRaw(ctx context.Context, networkID string) ([]byte, error) {
	path := fmt.Sprintf("/networks/%s/topology/linkLayer", networkID)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	return body, err
}

// GetSwitchPortsRaw retrieves the raw JSON bytes for all ports on a switch.
func (m *MerakiClient) GetSwitchPortsRaw(ctx context.Context, serial string) ([]byte, error) {
	path := fmt.Sprintf("/devices/%s/switch/ports", serial)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	return body, err
}

// NetworkLinkAggregation represents a network-level link aggregation group.
type NetworkLinkAggregation struct {
	ID          string `json:"id"`
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

// LLDPCDPData holds the LLDP/CDP neighbor data for a device.
type LLDPCDPData struct {
	// Ports maps port ID string → map of protocol ("lldp"/"cdp") → neighbor info
	Ports     map[string]map[string]interface{} `json:"ports"`
	SourceMac string                            `json:"sourceMac"`
}

// GetDeviceUplinkPorts returns the set of port IDs on the given device that are
// confirmed switch-to-switch uplinks according to LLDP/CDP neighbor data.
//
// Detection strategy (in priority order):
//  1. LLDP systemCapabilities contains "S-VLAN" or "Bridge" → confirmed switch uplink.
//     This is the most reliable signal; APs advertise "Two-port MAC Relay" instead.
//  2. CDP capabilities contains "Switch" AND NOT "Router" → confirmed switch uplink.
//     APs advertise "Router, Switch" so we require the absence of "Router" to exclude them.
//     Third-party switches (e.g. GTrans) that only advertise "Switch" are included.
//
// Ports absent from lldpCdp (e.g. uplinks to silent/non-LLDP devices) cannot be
// detected here and must be identified by other means (e.g. MAC table lookup).
// Returns an empty set (never nil) on error.
func (m *MerakiClient) GetDeviceUplinkPorts(ctx context.Context, serial string) map[string]struct{} {
	path := fmt.Sprintf("/devices/%s/lldpCdp", serial)
	body, _, err := m.doRequest(ctx, "GET", m.buildURL(path, nil))
	if err != nil {
		return map[string]struct{}{}
	}
	// The LLDP/CDP response ports field is:
	//   { "portId": { "cdp": {...}, "lldp": {...}, "deviceMac": "...", "device": {"url": "..."} } }
	// "device.url" and "cdp" are siblings at the port level, not nested inside each protocol.
	var raw struct {
		Ports map[string]map[string]json.RawMessage `json:"ports"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return map[string]struct{}{}
	}
	uplinks := make(map[string]struct{})
	for portID, portData := range raw.Ports {
		isSwitchNeighbor := false

		// Priority 1: LLDP systemCapabilities — most reliable; APs report "Two-port MAC Relay".
		if lldpRaw, ok := portData["lldp"]; ok {
			var lldp struct {
				SystemCapabilities string `json:"systemCapabilities"`
			}
			if json.Unmarshal(lldpRaw, &lldp) == nil {
				caps := lldp.SystemCapabilities
				if strings.Contains(caps, "S-VLAN") || strings.Contains(caps, "Bridge") {
					isSwitchNeighbor = true
				}
			}
		}

		// Priority 2: CDP capabilities — only if LLDP didn't already confirm.
		// Require "Switch" WITHOUT "Router" to exclude APs ("Router, Switch").
		if !isSwitchNeighbor {
			if cdpRaw, ok := portData["cdp"]; ok {
				var cdp struct {
					Capabilities string `json:"capabilities"`
				}
				if json.Unmarshal(cdpRaw, &cdp) == nil {
					caps := cdp.Capabilities
					if strings.Contains(caps, "Switch") && !strings.Contains(caps, "Router") {
						isSwitchNeighbor = true
					}
				}
			}
		}

		if isSwitchNeighbor {
			uplinks[portID] = struct{}{}
		}
	}
	return uplinks
}

// ResolveIPToMAC resolves an IP address to MAC address by querying Meraki clients API.
// Searches across multiple networks and returns the MAC, network ID, and hostname.
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

// parseLinkNext extracts the next page URL from a Link header.
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
