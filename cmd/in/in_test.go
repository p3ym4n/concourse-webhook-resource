package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/p3ym4n/concourse-webhook-resource/internal/models"
)

func TestWriteFiles(t *testing.T) {
	dest := t.TempDir()
	payload := &models.WebhookPayload{
		ID:        "test-uuid",
		Timestamp: "2024-01-01T00:00:00Z",
		Body: map[string]interface{}{
			"branch": "main",
			"count":  float64(42),
			"flag":   true,
			"nested": map[string]interface{}{"key": "val"},
		},
	}

	if err := writeFiles(dest, payload); err != nil {
		t.Fatalf("writeFiles: %v", err)
	}

	// version file
	got, _ := os.ReadFile(filepath.Join(dest, "version"))
	if string(got) != "test-uuid" {
		t.Errorf("version file: got %q, want %q", string(got), "test-uuid")
	}

	// params/ files
	branch, _ := os.ReadFile(filepath.Join(dest, "params", "branch"))
	if string(branch) != "main" {
		t.Errorf("params/branch: got %q, want %q", string(branch), "main")
	}
	count, _ := os.ReadFile(filepath.Join(dest, "params", "count"))
	if string(count) != "42" {
		t.Errorf("params/count: got %q, want %q", string(count), "42")
	}

	// payload.json exists
	if _, err := os.Stat(filepath.Join(dest, "payload.json")); err != nil {
		t.Error("payload.json not written")
	}

	// vars.yml exists
	if _, err := os.Stat(filepath.Join(dest, "vars.yml")); err != nil {
		t.Error("vars.yml not written")
	}
}

func TestWriteVarsYAML(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
		want []string // lines that must appear in the output
	}{
		{
			name: "string value",
			body: map[string]interface{}{"branch": "main"},
			want: []string{`branch: "main"`},
		},
		{
			name: "integer value",
			body: map[string]interface{}{"count": float64(7)},
			want: []string{"count: 7"},
		},
		{
			name: "float value",
			body: map[string]interface{}{"ratio": float64(1.5)},
			want: []string{"ratio: 1.5"},
		},
		{
			name: "bool value",
			body: map[string]interface{}{"flag": true},
			want: []string{"flag: true"},
		},
		{
			name: "null value",
			body: map[string]interface{}{"empty": nil},
			want: []string{"empty: ~"},
		},
		{
			name: "nested object becomes JSON string",
			body: map[string]interface{}{"meta": map[string]interface{}{"k": "v"}},
			want: []string{`meta: "{\"k\":\"v\"}"`},
		},
		{
			name: "string with special chars is escaped",
			body: map[string]interface{}{"msg": "say \"hello\""},
			want: []string{`msg: "say \"hello\""`},
		},
		{
			name: "keys are sorted alphabetically",
			body: map[string]interface{}{"z": "last", "a": "first"},
			want: []string{`a: "first"`, `z: "last"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vars.yml")
			if err := writeVarsYAML(path, tc.body); err != nil {
				t.Fatalf("writeVarsYAML: %v", err)
			}
			data, _ := os.ReadFile(path)
			content := string(data)
			for _, line := range tc.want {
				if !strings.Contains(content, line) {
					t.Errorf("vars.yml missing line %q\ngot:\n%s", line, content)
				}
			}
		})
	}
}

func TestValueToString(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
		{map[string]interface{}{"k": "v"}, `{"k":"v"}`},
	}
	for _, tc := range cases {
		got := valueToString(tc.in)
		if got != tc.want {
			t.Errorf("valueToString(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
