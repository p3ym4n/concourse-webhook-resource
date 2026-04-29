package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

// out is intentionally a no-op for this resource.
// It exists to satisfy the Concourse resource contract and returns a
// timestamp-based version so the implicit post-put get step can succeed.
func main() {
	log.SetOutput(os.Stderr)

	var req models.OutRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode out request: %v", err)
	}

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	resp := models.OutResponse{
		Version: models.Version{
			ID:        "manual-" + ts,
			Timestamp: ts,
		},
		Metadata: []models.Metadata{
			{Name: "type", Value: "manual"},
		},
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("failed to encode out response: %v", err)
	}
}
