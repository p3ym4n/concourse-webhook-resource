package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}

// handleWebhook accepts POST /webhook from external systems.
// Authentication: optional X-Webhook-Token header or ?token= query param.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.token != "" {
		incoming := r.Header.Get("X-Webhook-Token")
		if incoming == "" {
			incoming = r.URL.Query().Get("token")
		}
		if incoming != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	body := make(map[string]interface{})
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &body); err != nil {
			// Non-JSON body: store as raw string under "raw" key.
			body["raw"] = string(rawBody)
		}
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	payload := &models.WebhookPayload{
		ID:        newUUID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Body:      body,
		Headers:   headers,
	}

	if err := s.store.Save(payload); err != nil {
		http.Error(w, "failed to store payload", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": payload.ID}) //nolint:errcheck
}

// handleListPayloads serves GET /api/payloads?after=<RFC3339Nano> for the check script.
func (s *Server) handleListPayloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeInternal(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	after := r.URL.Query().Get("after")
	payloads, err := s.store.List(after)
	if err != nil {
		http.Error(w, "failed to list payloads", http.StatusInternalServerError)
		return
	}

	if payloads == nil {
		payloads = []*models.WebhookPayload{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payloads) //nolint:errcheck
}

// handlePayload serves GET and DELETE /api/payloads/:id for the in script.
func (s *Server) handlePayload(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/payloads/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if !s.authorizeInternal(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		payload, err := s.store.Get(id)
		if err != nil {
			http.Error(w, "payload not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck

	case http.MethodDelete:
		if err := s.store.Delete(id); err != nil {
			http.Error(w, "failed to delete payload", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// authorizeInternal validates the Bearer token on internal API calls.
// If no token is configured on the server, all requests are allowed.
func (s *Server) authorizeInternal(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == s.token
	}
	return r.Header.Get("X-Webhook-Token") == s.token
}

func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
