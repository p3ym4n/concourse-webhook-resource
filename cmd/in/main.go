package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

func main() {
	log.SetOutput(os.Stderr)

	if len(os.Args) < 2 {
		log.Fatal("usage: in <destination-dir>")
	}
	dest := os.Args[1]

	var req models.InRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode in request: %v", err)
	}

	payload, err := getPayload(req.Source, req.Version.ID)
	if err != nil {
		log.Fatalf("failed to fetch payload %s: %v", req.Version.ID, err)
	}

	if err := writeFiles(dest, payload); err != nil {
		log.Fatalf("failed to write output files: %v", err)
	}

	if req.Source.Cleanup {
		if err := deletePayload(req.Source, req.Version.ID); err != nil {
			log.Printf("warning: cleanup failed for %s: %v", req.Version.ID, err)
		}
	}

	metadata := buildMetadata(payload)
	resp := models.InResponse{
		Version:  req.Version,
		Metadata: metadata,
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		log.Fatalf("failed to encode in response: %v", err)
	}
}

// writeFiles writes the payload to the destination directory.
// Layout:
//
//	<dest>/payload.json        — full JSON body
//	<dest>/vars.yml            — flat YAML map for use with Concourse's load_var step
//	<dest>/params/<key>        — each top-level body field as a plain text file
//	<dest>/version             — the webhook ID
func writeFiles(dest string, payload *models.WebhookPayload) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// payload.json
	f, err := os.Create(filepath.Join(dest, "payload.json"))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// params/ directory — one file per top-level key
	paramsDir := filepath.Join(dest, "params")
	if err := os.MkdirAll(paramsDir, 0755); err != nil {
		return err
	}
	for key, val := range payload.Body {
		content := valueToString(val)
		if err := os.WriteFile(filepath.Join(paramsDir, key), []byte(content), 0644); err != nil {
			log.Printf("warning: could not write param file %q: %v", key, err)
		}
	}

	// vars.yml — for use with Concourse's load_var step
	if err := writeVarsYAML(filepath.Join(dest, "vars.yml"), payload.Body); err != nil {
		log.Printf("warning: could not write vars.yml: %v", err)
	}

	// version file
	return os.WriteFile(filepath.Join(dest, "version"), []byte(payload.ID), 0644)
}

// writeVarsYAML emits a flat YAML map of all top-level body fields.
// Scalar values (string, number, bool) are written as native YAML types.
// Complex values (objects, arrays) are JSON-encoded and written as quoted strings.
func writeVarsYAML(path string, body map[string]interface{}) error {
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(yamlQuoteKey(k))
		buf.WriteString(": ")
		buf.WriteString(yamlScalar(body[k]))
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// yamlQuoteKey quotes a YAML mapping key if it contains special characters.
func yamlQuoteKey(k string) string {
	if strings.ContainsAny(k, `: #{}[]|>&*!,%@` + "`\"'\\") || k == "" {
		return `"` + yamlEscape(k) + `"`
	}
	return k
}

// yamlScalar renders a JSON-decoded value as a YAML scalar.
func yamlScalar(v interface{}) string {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return "~"
	case string:
		return `"` + yamlEscape(val) + `"`
	default:
		data, _ := json.Marshal(val)
		return `"` + yamlEscape(string(data)) + `"`
	}
}

func yamlEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// buildMetadata returns sorted metadata pairs from top-level body fields.
func buildMetadata(payload *models.WebhookPayload) []models.Metadata {
	keys := make([]string, 0, len(payload.Body))
	for k := range payload.Body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	meta := []models.Metadata{
		{Name: "id", Value: payload.ID},
		{Name: "timestamp", Value: payload.Timestamp},
	}
	for _, k := range keys {
		meta = append(meta, models.Metadata{
			Name:  k,
			Value: valueToString(payload.Body[k]),
		})
	}
	return meta
}

// valueToString converts a JSON-decoded value to its string representation.
// Scalars become their natural string form; objects and arrays become JSON.
func valueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return ""
	default:
		data, _ := json.Marshal(val)
		return string(data)
	}
}

func getPayload(source models.Source, id string) (*models.WebhookPayload, error) {
	apiURL := fmt.Sprintf("%s/api/payloads/%s", source.URL, id)
	httpReq, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	if source.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+source.Token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("payload %q not found (already cleaned up?)", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var payload models.WebhookPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func deletePayload(source models.Source, id string) error {
	apiURL := fmt.Sprintf("%s/api/payloads/%s", source.URL, id)
	httpReq, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	if source.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+source.Token)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
