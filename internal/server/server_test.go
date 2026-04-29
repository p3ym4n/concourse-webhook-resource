package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
	"github.com/p3ym4n/concourse-webhook-resource/internal/server"
	"github.com/p3ym4n/concourse-webhook-resource/internal/storage"
)

func newTestServer(t *testing.T, token string) (*httptest.Server, *storage.FileStorage) {
	t.Helper()
	dir, err := os.MkdirTemp("", "webhook-server-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	store, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}

	srv := server.New(store, token)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, store
}

func postWebhook(t *testing.T, serverURL, token string, body interface{}) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, serverURL+"/webhook", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Webhook-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /webhook: %v", err)
	}
	return resp
}

func listPayloads(t *testing.T, serverURL, token, after string) []*models.WebhookPayload {
	t.Helper()
	u := serverURL + "/api/payloads"
	if after != "" {
		u += "?after=" + after
	}
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/payloads: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/payloads: unexpected status %d", resp.StatusCode)
	}
	var payloads []*models.WebhookPayload
	json.NewDecoder(resp.Body).Decode(&payloads)
	return payloads
}

// ── health ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── webhook intake ────────────────────────────────────────────────────────────

func TestWebhookNoAuth(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp := postWebhook(t, ts.URL, "", map[string]interface{}{"event": "push"})
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202, got %d", resp.StatusCode)
	}
}

func TestWebhookWithCorrectToken(t *testing.T) {
	ts, _ := newTestServer(t, "secret")
	resp := postWebhook(t, ts.URL, "secret", map[string]interface{}{"event": "push"})
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202, got %d", resp.StatusCode)
	}
}

func TestWebhookWithWrongToken(t *testing.T) {
	ts, _ := newTestServer(t, "secret")
	resp := postWebhook(t, ts.URL, "wrong", map[string]interface{}{"event": "push"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebhookMissingTokenRejected(t *testing.T) {
	ts, _ := newTestServer(t, "secret")
	resp := postWebhook(t, ts.URL, "", map[string]interface{}{"event": "push"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebhookTokenViaQueryParam(t *testing.T) {
	ts, _ := newTestServer(t, "secret")
	body, _ := json.Marshal(map[string]interface{}{"event": "push"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook?token=secret", bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 via query param token, got %d", resp.StatusCode)
	}
}

func TestWebhookNonJSONBody(t *testing.T) {
	ts, _ := newTestServer(t, "")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewBufferString("plain text payload"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 for non-JSON body, got %d", resp.StatusCode)
	}
	// Verify it stored as {"raw": "plain text payload"}
	payloads := listPayloads(t, ts.URL, "", "")
	if len(payloads) != 1 {
		t.Fatalf("expected 1 stored payload, got %d", len(payloads))
	}
	raw, ok := payloads[0].Body["raw"]
	if !ok {
		t.Error("expected 'raw' key in body for non-JSON payload")
	}
	if raw != "plain text payload" {
		t.Errorf("unexpected raw value: %v", raw)
	}
}

func TestWebhookMethodNotAllowed(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp, err := http.Get(ts.URL + "/webhook")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// ── API: list payloads ────────────────────────────────────────────────────────

func TestListPayloadsEmpty(t *testing.T) {
	ts, _ := newTestServer(t, "")
	payloads := listPayloads(t, ts.URL, "", "")
	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads, got %d", len(payloads))
	}
}

func TestListPayloadsAfterWebhook(t *testing.T) {
	ts, _ := newTestServer(t, "")
	postWebhook(t, ts.URL, "", map[string]interface{}{"event": "deploy"})
	postWebhook(t, ts.URL, "", map[string]interface{}{"event": "rollback"})

	payloads := listPayloads(t, ts.URL, "", "")
	if len(payloads) != 2 {
		t.Errorf("expected 2 payloads, got %d", len(payloads))
	}
}

func TestListPayloadsAfterTimestamp(t *testing.T) {
	ts, store := newTestServer(t, "")

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		p := &models.WebhookPayload{
			ID:        fmt.Sprintf("id-%d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano),
			Body:      map[string]interface{}{"i": i},
		}
		store.Save(p)
	}

	pivot := base.Add(1 * time.Minute).Format(time.RFC3339Nano)
	payloads := listPayloads(t, ts.URL, "", pivot)
	if len(payloads) != 2 {
		t.Errorf("expected 2 payloads after filter, got %d", len(payloads))
	}
}

func TestListPayloadsRequiresAuthWhenTokenSet(t *testing.T) {
	ts, _ := newTestServer(t, "secret")
	postWebhook(t, ts.URL, "secret", map[string]interface{}{"x": 1})

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/payloads", nil)
	// No auth header
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// ── API: get / delete specific payload ───────────────────────────────────────

func TestGetPayload(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp := postWebhook(t, ts.URL, "", map[string]interface{}{"ref": "main"})
	defer resp.Body.Close()

	var created map[string]string
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["id"]

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/payloads/"+id, nil)
	getResp, _ := http.DefaultClient.Do(req)
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for GET /api/payloads/%s, got %d", id, getResp.StatusCode)
	}
	defer getResp.Body.Close()

	var payload models.WebhookPayload
	json.NewDecoder(getResp.Body).Decode(&payload)
	if payload.ID != id {
		t.Errorf("expected payload ID %q, got %q", id, payload.ID)
	}
}

func TestGetPayloadNotFound(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp, _ := http.Get(ts.URL + "/api/payloads/does-not-exist")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeletePayload(t *testing.T) {
	ts, _ := newTestServer(t, "")
	resp := postWebhook(t, ts.URL, "", map[string]interface{}{"x": 1})
	defer resp.Body.Close()

	var created map[string]string
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["id"]

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/payloads/"+id, nil)
	delResp, _ := http.DefaultClient.Do(req)
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delResp.StatusCode)
	}

	// Confirm it's gone from the list
	payloads := listPayloads(t, ts.URL, "", "")
	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads after delete, got %d", len(payloads))
	}
}
