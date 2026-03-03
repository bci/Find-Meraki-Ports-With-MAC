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
	"fmt"
	"strings"
	"time"

	"Find-Meraki-Ports-With-MAC/pkg/filters"
	"Find-Meraki-Ports-With-MAC/pkg/logger"
	"Find-Meraki-Ports-With-MAC/pkg/macaddr"
	"Find-Meraki-Ports-With-MAC/pkg/meraki"
	"Find-Meraki-Ports-With-MAC/pkg/output"
)

// enrichPortInfoWithMembers calls the switch port API to get authoritative VLAN and port mode.
// Falls back to the provided defaults if the call fails or port is unsupported.
// For AGGR ports, it looks up VLAN/mode from the first resolvable member port
// (all member ports must be configured identically per Meraki requirements).
func enrichPortInfoWithMembers(ctx context.Context, client *meraki.MerakiClient, serial, portID string, aggrMembers []string, defaultVLAN int, defaultMode string) (vlan int, portMode string) {
	vlan, portMode = defaultVLAN, defaultMode
	if serial == "" || portID == "" || portID == "unknown" {
		return
	}
	// AGGR ports are link-aggregation virtual ports — no individual switch port API entry exists.
	// All member ports must be configured identically, so look up VLAN/mode from the first member.
	if strings.HasPrefix(portID, "AGGR") {
		for _, memberPort := range aggrMembers {
			if memberPort == "" {
				continue
			}
			sp, err := client.GetSwitchPort(ctx, serial, memberPort)
			if err != nil {
				continue
			}
			if sp.Type != "" {
				portMode = sp.Type
			}
			if sp.Vlan > 0 {
				vlan = sp.Vlan
			}
			return // all members are identical; first success is enough
		}
		return
	}
	sp, err := client.GetSwitchPort(ctx, serial, portID)
	if err != nil {
		return
	}
	if sp.Type != "" {
		portMode = sp.Type
	}
	if sp.Vlan > 0 {
		vlan = sp.Vlan
	}
	return
}

// parseAggrPort splits a raw Meraki AGGR port string into a clean port ID and member port list.
//
// Meraki MAC table entries encode link-aggregation ports as a compound string:
//
//	"AGGR/0=<serial>/<port>,<serial>/<port>,..."
//
// This function returns:
//   - cleanID: just "AGGR/0" (the part before the first '='), unchanged if no '='
//   - members: port numbers extracted from each "<serial>/<port>" segment (e.g. ["49","50","52"])
//
// If the string does not start with "AGGR" it is returned unchanged with nil members.
func parseAggrPort(raw string) (cleanID string, members []string) {
	if !strings.HasPrefix(raw, "AGGR") {
		return raw, nil
	}
	eqIdx := strings.IndexByte(raw, '=')
	if eqIdx < 0 {
		// No embedded member list — return as-is
		return raw, nil
	}
	cleanID = raw[:eqIdx]
	rest := raw[eqIdx+1:]
	for _, seg := range strings.Split(rest, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// Each segment is "<serial>/<portID>" — take the part after the last '/'
		if slashIdx := strings.LastIndexByte(seg, '/'); slashIdx >= 0 {
			if p := seg[slashIdx+1:]; p != "" {
				members = append(members, p)
			}
		}
	}
	if len(members) == 0 {
		members = nil
	}
	return cleanID, members
}

// resolveAggrPorts returns the physical member port IDs for an aggregation port (e.g. "AGGR/1").
// It first tries to parse member ports embedded in the raw port string (MAC table format), then
// falls back to querying the switch port list API via the provided cache.
// Returns nil if the port is not an AGGR port or members cannot be resolved.
func resolveAggrPorts(ctx context.Context, client *meraki.MerakiClient, serial, portID string, cache map[string]map[string][]string) []string {
	if !strings.HasPrefix(portID, "AGGR") {
		return nil
	}
	// If the portID contains embedded member info (raw MAC table format), parse it directly.
	if _, members := parseAggrPort(portID); members != nil {
		return members
	}
	// Otherwise fall back to the switch port list API.
	if _, ok := cache[serial]; !ok {
		cache[serial] = client.GetSwitchPortMembers(ctx, serial)
	}
	members := cache[serial][portID]
	if len(members) == 0 {
		return nil
	}
	return members
}

// isPortUplink returns true if portID is a confirmed uplink port for the given serial.
// For AGGR ports, it checks whether any member port is in the uplink set.
func isPortUplink(portID string, aggrMembers []string, uplinkSet map[string]struct{}) bool {
	if _, ok := uplinkSet[portID]; ok {
		return true
	}
	// For AGGR ports, check if any member port is an uplink.
	for _, m := range aggrMembers {
		if _, ok := uplinkSet[m]; ok {
			return true
		}
	}
	return false
}

func resolveDevices(cfg Config, macAddr, ipAddr string) ([]output.ResultRow, error) {
	log := newWebLogger()

	client := meraki.NewClient(cfg.APIKey, cfg.BaseURL, cfg.MaxRetries)
	ctx := context.Background()

	var targetOrg *meraki.Organization
	var targetNetwork *meraki.Network

	if cfg.OrgID != "" {
		// Fast path: org ID known, only fetch networks for that org
		networks, err := client.GetNetworks(ctx, cfg.OrgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get networks: %v", err)
		}
		for i, net := range networks {
			if net.ID == cfg.NetworkName {
				targetOrg = &meraki.Organization{ID: cfg.OrgID}
				targetNetwork = &networks[i]
				break
			}
		}
		if targetNetwork == nil {
			return nil, fmt.Errorf("network %s not found in org %s", cfg.NetworkName, cfg.OrgID)
		}
		// Fetch org name for display
		if orgs, err := client.GetOrganizations(ctx); err == nil {
			for _, o := range orgs {
				if o.ID == cfg.OrgID {
					targetOrg.Name = o.Name
					break
				}
			}
		}
	} else {
		// Slow path: search all orgs for the network (fallback when orgId not provided)
		orgs, err := client.GetOrganizations(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get organizations: %v", err)
		}
		if len(orgs) == 0 {
			return nil, fmt.Errorf("no organizations found")
		}
		for _, org := range orgs {
			networks, err := client.GetNetworks(ctx, org.ID)
			if err != nil {
				continue
			}
			for i, net := range networks {
				if net.ID == cfg.NetworkName {
					o := org
					targetOrg = &o
					targetNetwork = &networks[i]
					break
				}
			}
			if targetOrg != nil {
				break
			}
		}
		if targetOrg == nil || targetNetwork == nil {
			return nil, fmt.Errorf("network not found")
		}
	}

	log.Infof("Resolving in organization: %s, network: %s", targetOrg.Name, targetNetwork.Name)

	// Build MAC matcher
	var matcher func(string) bool
	var resolvedHostname string

	if ipAddr != "" {
		// IP resolution mode
		log.Debugf("Resolving IP: %s", ipAddr)

		resolvedMAC, _, hostname, err := client.ResolveIPToMAC(ctx, targetOrg.ID, []meraki.Network{*targetNetwork}, ipAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve IP %s: %v", ipAddr, err)
		}

		log.Debugf("Resolved IP %s to MAC %s (hostname: %s)", ipAddr, resolvedMAC, hostname)
		resolvedHostname = hostname

		matcher, _, _, err = macaddr.BuildMacMatcher(resolvedMAC)
		if err != nil {
			return nil, err
		}
	} else {
		// MAC mode
		var err error
		matcher, _, _, err = macaddr.BuildMacMatcher(macAddr)
		if err != nil {
			return nil, err
		}
	}

	// Get devices and process results (simplified version)
	devices, err := client.GetDevices(ctx, targetNetwork.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %v", err)
	}

	switches := filters.FilterSwitches(devices)
	results, err := processSwitchesForResolution(ctx, client, targetOrg, targetNetwork, switches, matcher, resolvedHostname, cfg.MacTablePoll, log)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func processSwitchesForResolution(ctx context.Context, client *meraki.MerakiClient, org *meraki.Organization, network *meraki.Network, switches []meraki.Device, matcher func(string) bool, hostname string, macTablePoll int, log *logger.Logger) ([]output.ResultRow, error) {
	var results []output.ResultRow
	resultsIndex := make(map[string]struct{})

	// Get network clients
	networkClients, err := client.GetNetworkClients(ctx, network.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network clients: %v", err)
	}

	log.Debugf("Network clients API returned %d clients", len(networkClients))

	// Build MAC->IP/hostname/lastSeen maps from network clients for enrichment fallback.
	macToIPWeb := make(map[string]string, len(networkClients))
	macToLastSeenWeb := make(map[string]string, len(networkClients))
	macToHostnameWeb := make(map[string]string, len(networkClients))
	for _, nc := range networkClients {
		norm, err2 := macaddr.NormalizeExactMac(nc.MAC)
		if err2 != nil {
			continue
		}
		if nc.IP != "" {
			macToIPWeb[norm] = nc.IP
		}
		if nc.LastSeen != "" {
			if existing := macToLastSeenWeb[norm]; existing == "" || nc.LastSeen > existing {
				macToLastSeenWeb[norm] = nc.LastSeen
			}
		}
		if hn := meraki.ClientHostname(nc); hn != "" {
			macToHostnameWeb[norm] = hn
		}
	}
	serialArpCacheWeb := make(map[string]map[string]string)
	resolveIP := func(normMAC, knownIP, serial string) (string, string) {
		ip := knownIP
		if ip == "" {
			ip = macToIPWeb[normMAC]
		}
		if ip == "" && serial != "" {
			if _, cached := serialArpCacheWeb[serial]; !cached {
				serialArpCacheWeb[serial] = client.FetchArpMap(ctx, serial, macTablePoll)
			}
			ip = serialArpCacheWeb[serial][normMAC]
		}
		hn := hostname
		if hn == "" {
			hn = macToHostnameWeb[normMAC]
		}
		if hn == "" && ip != "" {
			if hn = meraki.LookupHostOverride(ip, org.Name, network.Name); hn == "" {
				hn, _ = meraki.ResolveHostname(ip)
			}
		}
		return ip, hn
	}

	// Build device lookup map
	deviceBySerial := make(map[string]meraki.Device)
	for _, dev := range switches {
		deviceBySerial[dev.Serial] = dev
	}

	// AGGR member cache: serial → (aggrPortID → []memberPortIDs)
	// Pre-populate from the network-level link aggregations API, which is the only
	// reliable source when MAC table entries use the clean "AGGR/0" format (no embedded ports).
	// The per-device /switch/ports API does not expose linkAggregationId on this hardware.
	aggrCache := client.GetNetworkLinkAggregations(ctx, network.ID)

	// Build uplink port set using LLDP/CDP data per switch.
	// Ports where the neighbor has a Meraki dashboard URL are confirmed inter-device uplinks.
	// The topology/linkLayer API does not include port IDs on this firmware, so LLDP/CDP is used instead.
	// Results are cached per serial since we query each switch once.
	uplinkPortCache := make(map[string]map[string]struct{}) // serial → set of uplink portIDs
	getUplinkPorts := func(serial string) map[string]struct{} {
		if _, ok := uplinkPortCache[serial]; !ok {
			uplinkPortCache[serial] = client.GetDeviceUplinkPorts(ctx, serial)
		}
		return uplinkPortCache[serial]
	}

	// Process network clients
	for _, c := range networkClients {
		normMAC, err := macaddr.NormalizeExactMac(c.MAC)
		if err != nil {
			continue
		}
		if matcher(normMAC) {
			serial := strings.TrimSpace(c.RecentDeviceSerial)
			if serial == "" {
				continue
			}

			dev := deviceBySerial[serial]
			switchName := firstNonEmpty(dev.Name, c.RecentDeviceName, serial)

			port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
			aggrMembers := resolveAggrPorts(ctx, client, serial, port, aggrCache)
			vlan, portMode := enrichPortInfoWithMembers(ctx, client, serial, port, aggrMembers, 0, "")
			ip, hn := resolveIP(normMAC, c.IP, serial)

			addResult(resultsIndex, &results, output.ResultRow{
				OrgName:      org.Name,
				NetworkName:  network.Name,
				SwitchName:   switchName,
				SwitchSerial: serial,
				Port:         port,
				AggrPorts:    aggrMembers,
				MAC:          macaddr.FormatMacColon(normMAC),
				IP:           ip,
				Hostname:     hn,
				LastSeen:     firstNonEmpty(c.LastSeen, macToLastSeenWeb[normMAC]),
				VLAN:         vlan,
				PortMode:     portMode,
				IsUplink:     isPortUplink(port, aggrMembers, getUplinkPorts(serial)),
			})
		}
	}

	// Process device-level clients for each switch
	for _, dev := range switches {
		log.Debugf("Querying switch: %s (%s)", firstNonEmpty(dev.Name, dev.Serial), dev.Serial)

		// Try live MAC table lookup with up to 15 retries (30 seconds)
		macTableID, err := client.CreateMacTableLookup(ctx, dev.Serial)
		if err != nil {
			log.Debugf("MAC table lookup not available for %s: %v", dev.Serial, err)
			goto fallbackClients
		}

		{
			var macEntries []map[string]interface{}
			var status string
			for attempt := 0; attempt < macTablePoll; attempt++ {
				time.Sleep(2 * time.Second)
				macEntries, status, err = client.GetMacTableLookup(ctx, dev.Serial, macTableID)
				if err != nil {
					break
				}
				if status == "complete" {
					break
				}
				log.Debugf("MAC table status for %s: %s (attempt %d/%d)", firstNonEmpty(dev.Name, dev.Serial), status, attempt+1, macTablePoll)
			}

			if status == "complete" && len(macEntries) > 0 {
				foundInTable := false
				for _, entry := range macEntries {
					macStr, _ := entry["mac"].(string)
					if macStr == "" {
						continue
					}
					normMAC, err := macaddr.NormalizeExactMac(macStr)
					if err != nil {
						continue
					}
					if !matcher(normMAC) {
						continue
					}
					portID, _ := entry["portId"].(string)
					if portID == "" {
						portID, _ = entry["port"].(string)
					}
					if portID == "" {
						portID, _ = entry["interface"].(string)
					}
					vlan, _ := entry["vlan"].(float64)
					portMode, _ := entry["type"].(string)
					// Normalize AGGR raw strings (e.g. "AGGR/0=serial/49,...") to clean ID + member list
					cleanPortID, aggrMembers := parseAggrPort(firstNonEmpty(portID, "unknown"))
					if aggrMembers == nil {
						aggrMembers = resolveAggrPorts(ctx, client, dev.Serial, cleanPortID, aggrCache)
					}
					richVLAN, richMode := enrichPortInfoWithMembers(ctx, client, dev.Serial, cleanPortID, aggrMembers, int(vlan), portMode)
					ip, hn := resolveIP(normMAC, "", dev.Serial)
					addResult(resultsIndex, &results, output.ResultRow{
						OrgName:      org.Name,
						NetworkName:  network.Name,
						SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
						SwitchSerial: dev.Serial,
						Port:         cleanPortID,
						AggrPorts:    aggrMembers,
						MAC:          macaddr.FormatMacColon(normMAC),
						IP:           ip,
						Hostname:     hn,
						LastSeen:     macToLastSeenWeb[normMAC],
						VLAN:         richVLAN,
						PortMode:     richMode,
						IsUplink:     isPortUplink(cleanPortID, aggrMembers, getUplinkPorts(dev.Serial)),
					})
					foundInTable = true
				}
				// Only skip device-clients fallback if the target MAC was actually
				// found in the table. If the table had entries but our MAC wasn't
				// present (device temporarily inactive), fall through so device
				// clients history can still surface the result.
				if foundInTable {
					continue // skip fallback
				}
			}
		}

	fallbackClients:
		// Fallback to device clients API
		clients, err := client.GetDeviceClients(ctx, dev.Serial)
		if err != nil {
			log.Debugf("Failed to get device clients for %s: %v", dev.Serial, err)
			continue
		}
		for _, c := range clients {
			normMAC, err := macaddr.NormalizeExactMac(c.MAC)
			if err != nil || !matcher(normMAC) {
				continue
			}
			port := firstNonEmpty(c.SwitchportName, c.Switchport, c.Port, "unknown")
			aggrMembers3 := resolveAggrPorts(ctx, client, dev.Serial, port, aggrCache)
			vlan, portMode := enrichPortInfoWithMembers(ctx, client, dev.Serial, port, aggrMembers3, 0, "")
			ip, hn := resolveIP(normMAC, "", dev.Serial)
			addResult(resultsIndex, &results, output.ResultRow{
				OrgName:      org.Name,
				NetworkName:  network.Name,
				SwitchName:   firstNonEmpty(dev.Name, dev.Serial),
				SwitchSerial: dev.Serial,
				Port:         port,
				AggrPorts:    aggrMembers3,
				MAC:          macaddr.FormatMacColon(normMAC),
				IP:           ip,
				Hostname:     hn,
				LastSeen:     c.LastSeen,
				VLAN:         vlan,
				PortMode:     portMode,
				IsUplink:     isPortUplink(port, aggrMembers3, getUplinkPorts(dev.Serial)),
			})
		}
	}

	return results, nil
}
