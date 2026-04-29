package storage_test

import (
	"os"
	"testing"
	"time"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
	"github.com/p3ym4n/concourse-webhook-resource/internal/storage"
)

func newStore(t *testing.T) *storage.FileStorage {
	t.Helper()
	dir, err := os.MkdirTemp("", "webhook-storage-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	store, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func makePayload(id, ts string) *models.WebhookPayload {
	return &models.WebhookPayload{
		ID:        id,
		Timestamp: ts,
		Body:      map[string]interface{}{"key": "value", "id": id},
	}
}

func TestSaveAndGet(t *testing.T) {
	store := newStore(t)
	p := makePayload("abc123", time.Now().UTC().Format(time.RFC3339Nano))

	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get("abc123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("got ID %q, want %q", got.ID, p.ID)
	}
}

func TestGetNotFound(t *testing.T) {
	store := newStore(t)
	if _, err := store.Get("nonexistent"); err == nil {
		t.Error("expected error for missing payload, got nil")
	}
}

func TestListEmpty(t *testing.T) {
	store := newStore(t)
	payloads, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("expected 0 payloads, got %d", len(payloads))
	}
}

func TestListAll(t *testing.T) {
	store := newStore(t)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		p := makePayload("id-"+string(rune('a'+i)), ts)
		if err := store.Save(p); err != nil {
			t.Fatal(err)
		}
	}

	payloads, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(payloads) != 3 {
		t.Errorf("expected 3 payloads, got %d", len(payloads))
	}
	// Verify chronological order
	for i := 1; i < len(payloads); i++ {
		if payloads[i].Timestamp <= payloads[i-1].Timestamp {
			t.Errorf("payloads not in chronological order at index %d", i)
		}
	}
}

func TestListAfterTimestamp(t *testing.T) {
	store := newStore(t)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	var timestamps []string
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		timestamps = append(timestamps, ts)
		p := makePayload("id-"+string(rune('a'+i)), ts)
		if err := store.Save(p); err != nil {
			t.Fatal(err)
		}
	}

	// List after the 2nd timestamp — should return the 3rd, 4th, 5th
	payloads, err := store.List(timestamps[1])
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(payloads) != 3 {
		t.Errorf("expected 3 payloads after filter, got %d", len(payloads))
	}
	for _, p := range payloads {
		if p.Timestamp <= timestamps[1] {
			t.Errorf("payload %s has timestamp %s which is not after %s", p.ID, p.Timestamp, timestamps[1])
		}
	}
}

func TestDelete(t *testing.T) {
	store := newStore(t)
	p := makePayload("del-me", time.Now().UTC().Format(time.RFC3339Nano))

	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("del-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("del-me"); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestDeleteIdempotent(t *testing.T) {
	store := newStore(t)
	// Deleting a non-existent payload should not error
	if err := store.Delete("ghost"); err != nil {
		t.Errorf("Delete of non-existent payload returned error: %v", err)
	}
}

func TestListExcludesDeleted(t *testing.T) {
	store := newStore(t)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		p := makePayload("id-"+string(rune('a'+i)), ts)
		if err := store.Save(p); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.Delete("id-a"); err != nil {
		t.Fatal(err)
	}

	payloads, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 2 {
		t.Errorf("expected 2 payloads after deletion, got %d", len(payloads))
	}
	for _, p := range payloads {
		if p.ID == "id-a" {
			t.Error("deleted payload still appears in List")
		}
	}
}
