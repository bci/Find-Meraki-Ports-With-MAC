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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestHandleTopology_EscapesQueryValues checks that query values copied into
// the page's meta tags are HTML-escaped. Without escaping, a value such as a
// DHCP-supplied hostname can close the content attribute and inject markup
// (reflected XSS).
func TestHandleTopology_EscapesQueryValues(t *testing.T) {
	const hostile = `"><script>alert(1)</script>`
	q := url.Values{}
	for _, k := range []string{"networkId", "orgId", "apiKey", "highlightSerial", "highlightPort",
		"highlightName", "portMode", "mac", "hostname"} {
		q.Set(k, hostile)
	}
	rec := httptest.NewRecorder()
	handleTopology(rec, httptest.NewRequest(http.MethodGet, "/topology?"+q.Encode(), nil))

	body := rec.Body.String()
	if strings.Contains(body, hostile) {
		t.Fatalf("topology page contains the raw hostile value; meta content is not escaped")
	}
	if got := strings.Count(body, `content="&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;"`); got != 9 {
		t.Errorf("escaped value appears %d times, want 9 (one per meta tag)", got)
	}
}

// TestHandleWebSocketLogs_RejectsCrossOrigin checks that /ws/logs refuses a
// WebSocket handshake from another origin. Browsers let any web page open a
// WebSocket to localhost, so accepting every origin would let any site the
// user visits read the live log stream (MACs, IPs, hostnames).
func TestHandleWebSocketLogs_RejectsCrossOrigin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(handleWebSocketLogs))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	h := http.Header{}
	h.Set("Origin", "https://evil.example")
	if conn, _, err := websocket.DefaultDialer.Dial(wsURL, h); err == nil {
		_ = conn.Close()
		t.Fatal("cross-origin WebSocket handshake succeeded, want it rejected")
	}

	h.Set("Origin", srv.URL) // same origin, as the UI connects from the page itself
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, h)
	if err != nil {
		t.Fatalf("same-origin WebSocket handshake failed: %v", err)
	}
	_ = conn.Close()
}
