package requirements

import (
	"fmt"

	"github.com/blueai2022/nucleus/internal/session"
)

// Metadata contains requirement-specific configuration
type Metadata struct {
	Code              string   // Formal requirement code (e.g., "REQ-123")
	ReferencedModules []string // Template modules (e.g., ["metrics", "connect"])
	Name              string
	TargetFile        string
	ExampleDirs       []string
	PromptTemplate    string
	Language          session.Language
}

// Registry manages requirement metadata
type Registry interface {
	Lookup(projectID, requirementCode string) (*Metadata, error)
}

type registry struct {
	metadata map[string]*Metadata
}

// NewRegistry creates a new requirement registry
func NewRegistry() Registry {
	return &registry{
		metadata: map[string]*Metadata{
			// Key format: projectID:requirementCode
			"test-001:REQ-123": {
				Code:              "REQ-123",
				ReferencedModules: []string{"metrics"},
				Name:              "NATS Metrics Implementation",
				TargetFile:        "nats/metrics.go",
				ExampleDirs:       []string{"pkg/examples/metrics"},
				PromptTemplate: `Implement NATS publish latency tracking with p50/p95/p99 percentiles.

Follow the patterns in pkg/examples/metrics/, include prior PR Review note in pr_review_notes.txt in the same directory.
In terms of the requirement scope, think like a PR reviewer: ensure a complete implementation so that the new added feature is self-complete.`,
				Language: session.LanguageGo,
			},
			"test-001:REQ-456": {
				Code:              "REQ-456",
				ReferencedModules: []string{"metrics", "connect"},
				Name:              "NATS with Metrics",
				TargetFile:        "nats/client.go",
				ExampleDirs:       []string{"pkg/examples/metrics", "pkg/examples/connect"},
				PromptTemplate:    `Implement NATS client with metrics...`,
				Language:          session.LanguageGo,
			},
			"test-001:metrics": {
				Code:        "metrics",
				Name:        "NATS Metrics Implementation",
				TargetFile:  "nats/metrics.go",
				ExampleDirs: []string{"pkg/examples/metrics"},
				PromptTemplate: `Implement NATS publish latency tracking with p50/p95/p99 percentiles.

Follow the patterns in pkg/examples/metrics/, include prior PR Review note in pr_review_notes.txt in the same directory.
In terms of the requirement scope, think like a PR reviewer: ensure a complete implementation so that the new added feature is self-complete.`,
				Language: session.LanguageGo,
			},
		},
	}
}

func (r *registry) Lookup(projectID, requirementCode string) (*Metadata, error) {
	key := fmt.Sprintf("%s:%s", projectID, requirementCode)
	meta, exists := r.metadata[key]
	if !exists {
		return nil, fmt.Errorf("requirement not found: %s (tried key: %s)", requirementCode, key)
	}
	return meta, nil
}
