package models

// Source is the resource-level configuration set in the pipeline.
type Source struct {
	URL         string `json:"url"`
	Token       string `json:"token,omitempty"`
	TokenHeader string `json:"token_header,omitempty"` // default: X-Webhook-Token
	Cleanup     bool   `json:"cleanup,omitempty"`      // delete payload after in fetches it
}

func (s Source) TokenHeaderName() string {
	if s.TokenHeader != "" {
		return s.TokenHeader
	}
	return "X-Webhook-Token"
}

// Version uniquely identifies a single webhook invocation.
type Version struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// Metadata is a key/value pair surfaced in the Concourse UI.
type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CheckRequest is the stdin payload for /opt/resource/check.
type CheckRequest struct {
	Source  Source   `json:"source"`
	Version *Version `json:"version"`
}

// InRequest is the stdin payload for /opt/resource/in.
type InRequest struct {
	Source  Source   `json:"source"`
	Version Version  `json:"version"`
	Params  InParams `json:"params,omitempty"`
}

// InParams are per-get-step parameters (currently unused, reserved for future use).
type InParams struct{}

// InResponse is the stdout payload for /opt/resource/in.
type InResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
}

// OutRequest is the stdin payload for /opt/resource/out.
type OutRequest struct {
	Source Source    `json:"source"`
	Params OutParams `json:"params,omitempty"`
}

// OutParams are per-put-step parameters (currently unused).
type OutParams struct{}

// OutResponse is the stdout payload for /opt/resource/out.
type OutResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
}

// WebhookPayload is a single received webhook event stored by the server.
type WebhookPayload struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"` // RFC3339Nano UTC
	Body      map[string]interface{} `json:"body"`
	Headers   map[string]string      `json:"headers,omitempty"`
}
