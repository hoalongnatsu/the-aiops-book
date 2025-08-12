package types

import (
	"time"
)

// ServerInfo contains metadata about our MCP server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ResourceInfo represents an MCP resource that AI systems can access
type ResourceInfo struct {
	URI         string            `json:"uri"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	MimeType    string            `json:"mimeType"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AWSResource represents AWS infrastructure resources
type AWSResource struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Region   string                 `json:"region"`
	State    string                 `json:"state"`
	Tags     map[string]string      `json:"tags,omitempty"`
	Details  map[string]interface{} `json:"details"`
	LastSeen time.Time              `json:"lastSeen"`
}
