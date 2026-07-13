package task

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenericWebhookNotification(t *testing.T) {
	var received genericWebhookPayload
	var signature string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		signature = request.Header.Get("X-PST-Signature")
		mac := hmac.New(sha256.New, []byte("secret"))
		_, _ = mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if signature != want {
			t.Errorf("unexpected signature %q, want %q", signature, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := newWebhookNotifier(true)
	notifier.client = server.Client()
	message := NotificationMessage{
		Event:      EventTaskFailed,
		OccurredAt: time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC),
		Title:      "Task failed",
		Message:    "Nightly restart failed",
	}
	err := notifier.Send(context.Background(), NotificationSettings{
		Provider:       NotificationGeneric,
		WebhookURL:     server.URL,
		Secret:         "secret",
		TimeoutSeconds: 5,
	}, message)
	if err != nil {
		t.Fatal(err)
	}
	if received.Schema != 1 || received.Event != EventTaskFailed {
		t.Fatalf("unexpected generic payload: %#v", received)
	}
	if signature == "" {
		t.Fatal("generic webhook signature is missing")
	}
}

func TestDiscordWebhookNotification(t *testing.T) {
	var received discordWebhookPayload
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-PST-Signature") != "" {
			t.Error("Discord payload should not include a generic signature")
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := newWebhookNotifier(true)
	notifier.client = server.Client()
	err := notifier.Send(context.Background(), NotificationSettings{
		Provider:       NotificationDiscord,
		WebhookURL:     server.URL,
		TimeoutSeconds: 5,
	}, NotificationMessage{
		Event:   EventWatchdogRecovered,
		Title:   "Server recovered",
		Message: "Palworld is online again.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.Username != "Palworld Server Tool" || !strings.Contains(received.Content, "Server recovered") {
		t.Fatalf("unexpected Discord payload: %#v", received)
	}
	if received.AllowedMentions.Parse == nil || len(received.AllowedMentions.Parse) != 0 {
		t.Fatalf("Discord mentions were not disabled: %#v", received.AllowedMentions)
	}
}

func TestSafeWebhookClientRejectsPrivateAddresses(t *testing.T) {
	notifier := newWebhookNotifier(false)
	client := notifier.safeHTTPClient(time.Second)
	request, err := http.NewRequest(http.MethodPost, "https://127.0.0.1:443/hook", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(request)
	if err == nil || !strings.Contains(err.Error(), "private or reserved") {
		t.Fatalf("private webhook address was not rejected: %v", err)
	}
}

func TestWebhookHelpers(t *testing.T) {
	for _, address := range []string{
		"10.0.0.1",
		"100.64.0.1",
		"192.0.2.1",
		"198.18.0.1",
		"203.0.113.1",
		"2001:db8::1",
		"2002:0a00:0001::1",
	} {
		if !isDisallowedWebhookIP(net.ParseIP(address)) {
			t.Fatalf("private or reserved webhook address %s was accepted", address)
		}
	}
	for _, address := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if isDisallowedWebhookIP(net.ParseIP(address)) {
			t.Fatalf("public webhook address %s was rejected", address)
		}
	}
	if preview := webhookPreview("https://discord.com/api/webhooks/id/token"); preview != "https://discord.com/…" {
		t.Fatalf("unexpected webhook preview: %q", preview)
	}
}
