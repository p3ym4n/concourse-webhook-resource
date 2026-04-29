package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

func main() {
	log.SetOutput(os.Stderr)

	var req models.CheckRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode check request: %v", err)
	}

	afterTimestamp := ""
	if req.Version != nil {
		afterTimestamp = req.Version.Timestamp
	}

	payloads, err := listPayloads(req.Source, afterTimestamp)
	if err != nil {
		log.Fatalf("failed to list payloads: %v", err)
	}

	versions := make([]models.Version, 0, len(payloads))
	for _, p := range payloads {
		versions = append(versions, models.Version{
			ID:        p.ID,
			Timestamp: p.Timestamp,
		})
	}

	// On the very first check (no prior version), return at most the latest payload
	// to avoid flooding the build queue with a backlog of old webhooks.
	if req.Version == nil && len(versions) > 1 {
		versions = versions[len(versions)-1:]
	}

	if err := json.NewEncoder(os.Stdout).Encode(versions); err != nil {
		log.Fatalf("failed to encode check response: %v", err)
	}
}

func listPayloads(source models.Source, afterTimestamp string) ([]*models.WebhookPayload, error) {
	apiURL := source.URL + "/api/payloads"
	if afterTimestamp != "" {
		apiURL += "?after=" + url.QueryEscape(afterTimestamp)
	}

	httpReq, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if source.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+source.Token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var payloads []*models.WebhookPayload
	if err := json.NewDecoder(resp.Body).Decode(&payloads); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return payloads, nil
}
