package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

// FileStorage persists each webhook payload as an individual JSON file under dir.
type FileStorage struct {
	dir string
	mu  sync.Mutex
}

func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating storage dir %q: %w", dir, err)
	}
	return &FileStorage{dir: dir}, nil
}

func (s *FileStorage) Save(payload *models.WebhookPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, payload.ID+".json")
	return os.WriteFile(path, data, 0644)
}

func (s *FileStorage) List(afterTimestamp string) ([]*models.WebhookPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("reading storage dir: %w", err)
	}

	var payloads []*models.WebhookPayload
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var p models.WebhookPayload
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if afterTimestamp == "" || p.Timestamp > afterTimestamp {
			payloads = append(payloads, &p)
		}
	}

	sort.Slice(payloads, func(i, j int) bool {
		return payloads[i].Timestamp < payloads[j].Timestamp
	})
	return payloads, nil
}

func (s *FileStorage) Get(id string) (*models.WebhookPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("payload %q not found: %w", id, err)
	}
	var p models.WebhookPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing payload %q: %w", id, err)
	}
	return &p, nil
}

func (s *FileStorage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting payload %q: %w", id, err)
	}
	return nil
}
