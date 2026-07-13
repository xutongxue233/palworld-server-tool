package fleet

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoadConfigurationValidatesIsolatedNodes(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("fleet.node_id", "primary")
	viper.Set("fleet.node_name", "Primary World")
	viper.Set("fleet.node_token", "primary-primary-primary-primary-1234")
	viper.Set("fleet.timeout_seconds", 20)
	viper.Set("fleet.nodes", []map[string]any{
		{
			"id":                    "second-world",
			"name":                  "Second World",
			"base_url":              "http://127.0.0.1:18081/",
			"token":                 "second-second-second-second-1234",
			"allow_private_network": true,
		},
	})

	configuration := LoadConfiguration()
	if len(configuration.Issues) != 0 || len(configuration.Nodes) != 1 {
		t.Fatalf("unexpected fleet configuration: %#v", configuration)
	}
	node := configuration.Nodes[0]
	if node.ID != "second-world" || node.Name != "Second World" ||
		node.BaseURL != "http://127.0.0.1:18081" || node.Timeout.Seconds() != 20 {
		t.Fatalf("unexpected normalized node: %#v", node)
	}
	if strings.Contains(strings.TrimSpace(mustJSON(t, configuration)), node.Token) {
		t.Fatal("configuration serialization must not be used to expose node tokens")
	}
}

func TestLoadConfigurationRejectsDuplicatesWeakTokensAndUnsafeHTTP(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("fleet.node_id", "primary")
	viper.Set("fleet.nodes", []map[string]any{
		{
			"id":       "primary",
			"base_url": "https://one.example.com",
			"token":    "one-one-one-one-one-one-one-1234",
		},
		{
			"id":       "weak",
			"base_url": "https://two.example.com",
			"token":    "short",
		},
		{
			"id":       "plain-http",
			"base_url": "http://192.168.1.5:8080",
			"token":    "plain-http-plain-http-token-1234",
		},
	})

	configuration := LoadConfiguration()
	if len(configuration.Nodes) != 0 {
		t.Fatalf("invalid nodes were accepted: %#v", configuration.Nodes)
	}
	for _, code := range []string{
		"fleet_node_id_duplicate",
		"fleet_node_token_invalid",
		"fleet_node_http_requires_opt_in",
	} {
		assertFleetIssue(t, configuration.Issues, code)
	}
}

func TestFleetHTTPClientRequiresExplicitPrivateNetworkOptIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte("ok"))
	}))
	defer server.Close()

	blocked := NodeConfig{BaseURL: server.URL, Timeout: normalizedFleetTimeout(2)}
	blockedURL, err := BuildNodeURL(blocked, "/api/server", "")
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, blockedURL, nil)
	client := NewHTTPClient(blocked, blocked.Timeout)
	_, err = client.Do(request)
	client.CloseIdleConnections()
	if err == nil || !strings.Contains(err.Error(), "disallowed address") {
		t.Fatalf("loopback node was not blocked without opt-in: %v", err)
	}

	allowed := blocked
	allowed.AllowPrivateNetwork = true
	client = NewHTTPClient(allowed, allowed.Timeout)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "ok" {
		t.Fatalf("unexpected private node response: %q", body)
	}
}

func TestFleetHTTPClientBoundsResponseBodyAndTriesValidatedAddresses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusOK)
		if flusher, ok := response.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(200 * time.Millisecond)
		_, _ = response.Write([]byte("late"))
	}))
	defer server.Close()

	node := NodeConfig{
		BaseURL:             server.URL,
		AllowPrivateNetwork: true,
		Timeout:             2 * time.Second,
	}
	client := NewHTTPClient(node, 50*time.Millisecond)
	response, err := client.Get(server.URL)
	if err == nil {
		defer response.Body.Close()
		_, err = io.ReadAll(response.Body)
	}
	client.CloseIdleConnections()
	if err == nil {
		t.Fatal("fleet client did not enforce its total response timeout")
	}

	_, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := dialFleetAddresses(
		context.Background(),
		&net.Dialer{Timeout: time.Second},
		"tcp",
		port,
		[]net.IPAddr{
			{IP: net.ParseIP("127.0.0.2")},
			{IP: net.ParseIP("127.0.0.1")},
		},
	)
	if err != nil {
		t.Fatalf("fleet client did not try a second validated address: %v", err)
	}
	_ = connection.Close()
}

func assertFleetIssue(t *testing.T, issues []ConfigIssue, code string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("fleet issue %q not found in %#v", code, issues)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
