package storage

import "github.com/p3ym4n/concourse-webhook-resource/internal/models"

// Storage persists and retrieves webhook payloads.
type Storage interface {
	Save(payload *models.WebhookPayload) error
	// List returns all payloads with Timestamp strictly after afterTimestamp (RFC3339Nano).
	// Pass "" to list all payloads.
	List(afterTimestamp string) ([]*models.WebhookPayload, error)
	Get(id string) (*models.WebhookPayload, error)
	Delete(id string) error
}
