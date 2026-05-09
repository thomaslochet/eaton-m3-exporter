package m3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientAuthenticateAndFetchSnapshot(t *testing.T) {
	t.Parallel()

	var tokenRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/mbdetnrs/2.0/oauth2/token/" {
			tokenRequests.Add(1)
			writeJSON(t, w, map[string]string{"token_type": "Bearer", "access_token": "token-1"})
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Fatalf("Authorization = %q, want Bearer token-1", got)
		}
		handleSnapshotPath(t, w, r.URL.Path)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	snapshot, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if tokenRequests.Load() != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests.Load())
	}
	if snapshot.PowerDistributions[0].Inputs[0].Measures.Voltage == nil || *snapshot.PowerDistributions[0].Inputs[0].Measures.Voltage != 244.6 {
		t.Fatalf("input voltage = %v, want 244.6", snapshot.PowerDistributions[0].Inputs[0].Measures.Voltage)
	}
	if snapshot.Suppliers[0].Summary.ProtectionCapacityPercent != 100 {
		t.Fatalf("protection capacity = %v, want 100", snapshot.Suppliers[0].Summary.ProtectionCapacityPercent)
	}
	if snapshot.Temperatures[0].Measure != 298.15 {
		t.Fatalf("temperature = %v, want 298.15", snapshot.Temperatures[0].Measure)
	}
}

func TestClientRetriesAfterUnauthorized(t *testing.T) {
	t.Parallel()

	var tokenRequests atomic.Int64
	var statusRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/mbdetnrs/2.0/oauth2/token/" {
			count := tokenRequests.Add(1)
			writeJSON(t, w, map[string]string{"token_type": "Bearer", "access_token": fmt.Sprintf("token-%d", count)})
			return
		}
		if r.URL.Path == "/rest/mbdetnrs/2.0/powerService/status" {
			if statusRequests.Add(1) == 1 {
				http.Error(w, "expired", http.StatusUnauthorized)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer token-2" {
				t.Fatalf("Authorization after retry = %q, want Bearer token-2", got)
			}
			writeJSON(t, w, Status{Operating: "in service", Health: "ok"})
			return
		}
		handleSnapshotPath(t, w, r.URL.Path)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if _, err := client.FetchSnapshot(context.Background()); err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if tokenRequests.Load() != 2 {
		t.Fatalf("token requests = %d, want 2", tokenRequests.Load())
	}
}

func TestClientReturnsNon2xxErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/mbdetnrs/2.0/oauth2/token/" {
			writeJSON(t, w, map[string]string{"token_type": "Bearer", "access_token": "token"})
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.FetchSnapshot(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Fatalf("FetchSnapshot() error = %v, want 500", err)
	}
}

func TestLastPathSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "absolute resource", in: "/rest/mbdetnrs/2.0/powerService/suppliers/abc", want: "abc"},
		{name: "trailing slash", in: "/rest/mbdetnrs/2.0/powerDistributions/1/", want: "1"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lastPathSegment(tt.in); got != tt.want {
				t.Fatalf("lastPathSegment(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := NewClient(Target{
		Name:       "ups-main",
		BaseURL:    baseURL,
		Username:   "admin",
		Password:   "secret",
		APIVersion: "2.0",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func handleSnapshotPath(t *testing.T, w http.ResponseWriter, p string) {
	t.Helper()
	switch p {
	case "/rest/mbdetnrs/2.0/powerService/status", "/rest/mbdetnrs/2.0/environmentService/status":
		writeJSON(t, w, Status{Operating: "in service", Health: "ok"})
	case "/rest/mbdetnrs/2.0/alarmService/parametricCount":
		writeJSON(t, w, AlarmCounts{ActiveCount: 2, TotalCount: 4})
	case "/rest/mbdetnrs/2.0/alarmService/mostCriticalAlarm":
		writeJSON(t, w, Alarm{ID: "6705", State: "open", Code: "80D", Level: "warning", Description: "Internal configuration failure"})
	case "/rest/mbdetnrs/2.0/powerDistributions":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/powerDistributions/1")
	case "/rest/mbdetnrs/2.0/powerDistributions/1/identification":
		writeJSON(t, w, Identification{UUID: "pd-uuid", FriendlyName: "UPS", SerialNumber: "S1"})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/status":
		writeJSON(t, w, Status{Operating: "in service", Health: "warning", ConfigurationFault: boolPtr(true)})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/backupSystem/powerBank/status":
		writeJSON(t, w, PowerBankStatus{Operating: "stopped", Health: "ok", Supplied: boolPtr(true)})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/inputs":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/powerDistributions/1/inputs/1")
	case "/rest/mbdetnrs/2.0/powerDistributions/1/outputs":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/powerDistributions/1/outputs/1")
	case "/rest/mbdetnrs/2.0/powerDistributions/1/outlets":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/powerDistributions/1/outlets/1")
	case "/rest/mbdetnrs/2.0/powerDistributions/1/inputs/1/identification":
		writeJSON(t, w, Identification{UUID: "input-uuid", FriendlyName: "Main utility"})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/outputs/1/identification":
		writeJSON(t, w, Identification{UUID: "output-uuid", FriendlyName: "Output"})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/outlets/1/identification":
		writeJSON(t, w, Identification{UUID: "outlet-uuid", FriendlyName: "Primary"})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/inputs/1/status", "/rest/mbdetnrs/2.0/powerDistributions/1/outputs/1/status", "/rest/mbdetnrs/2.0/powerDistributions/1/outlets/1/status":
		writeJSON(t, w, Status{Operating: "in service", Health: "ok", Supplied: boolPtr(true)})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/inputs/1/measures":
		writeJSON(t, w, Measures{Voltage: floatPtr(244.6), Current: floatPtr(0.1), Frequency: floatPtr(50)})
	case "/rest/mbdetnrs/2.0/powerDistributions/1/outputs/1/measures", "/rest/mbdetnrs/2.0/powerDistributions/1/outlets/1/measures":
		writeJSON(t, w, Measures{ActivePower: floatPtr(12), ApparentPower: floatPtr(15), Voltage: floatPtr(230)})
	case "/rest/mbdetnrs/2.0/powerService/suppliers":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/powerService/suppliers/supplier-1")
	case "/rest/mbdetnrs/2.0/powerService/suppliers/supplier-1/measures":
		writeJSON(t, w, Measures{Voltage: floatPtr(243.8), Frequency: floatPtr(49.9)})
	case "/rest/mbdetnrs/2.0/powerService/suppliers/supplier-1/summary":
		writeJSON(t, w, SupplierSummary{CapacityRuntime: 59940, ProtectionCapacityPercent: 100, Mode: "normal", Quality: "protecting"})
	case "/rest/mbdetnrs/2.0/environmentService/temperatures":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/environmentService/temperatures/temp-1")
	case "/rest/mbdetnrs/2.0/environmentService/temperatures/temp-1":
		writeJSON(t, w, EnvironmentSensor{ID: "temp-1", Name: "Sensor 1", Measure: 298.15, AlarmLevel: "good"})
	case "/rest/mbdetnrs/2.0/environmentService/humidities":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/environmentService/humidities/hum-1")
	case "/rest/mbdetnrs/2.0/environmentService/humidities/hum-1":
		writeJSON(t, w, EnvironmentSensor{ID: "hum-1", Name: "Sensor 1", Measure: 50, AlarmLevel: "good"})
	case "/rest/mbdetnrs/2.0/environmentService/inputs":
		writeMembers(t, w, "/rest/mbdetnrs/2.0/environmentService/inputs/input-1")
	case "/rest/mbdetnrs/2.0/environmentService/inputs/input-1":
		writeJSON(t, w, EnvironmentInput{ID: "input-1", Name: "Input 1.1", Active: true, AlarmLevel: "critical"})
	default:
		http.NotFound(w, nil)
	}
}

func writeMembers(t *testing.T, w http.ResponseWriter, ids ...string) {
	t.Helper()
	members := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		members = append(members, map[string]string{"@id": id})
	}
	writeJSON(t, w, map[string]any{"members@count": len(ids), "members": members})
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
