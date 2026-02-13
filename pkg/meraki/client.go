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
	"net/http"
	"net/url"
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
}

// MerakiClient is an HTTP client wrapper for the Meraki Dashboard API.
type MerakiClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new Meraki API client.
func NewClient(apiKey, baseURL string) *MerakiClient {
	if baseURL == "" {
		baseURL = "https://api.meraki.com/api/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &MerakiClient{
		apiKey:  apiKey,
		baseURL: baseURL,
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
	for attempt := 0; attempt < 6; attempt++ {
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
