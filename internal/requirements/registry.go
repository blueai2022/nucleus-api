package requirements

import (
	"fmt"

	"github.com/blueai2022/nucleus/internal/session"
)

// Metadata contains requirement-specific configuration
type Metadata struct {
	Code           string
	TargetFile     string
	ExampleDirs    []string
	PromptTemplate string
	Language       session.Language
}

// Registry manages requirement metadata
type Registry interface {
	Lookup(projectID, requirementCode string) (*Metadata, error)
}

type registry struct {
	// For now, hardcoded. Later: database, YAML configs, etc.
	metadata map[string]*Metadata
}

// NewRegistry creates a new requirement registry
func NewRegistry() Registry {
	return &registry{
		metadata: map[string]*Metadata{
			// Key format: projectID:requirementCode
			"test-001:REQ-123": {
				Code:        "REQ-123",
				TargetFile:  "nats/metrics.go",
				ExampleDirs: []string{"pkg/examples/metrics"},
				PromptTemplate: `Implement NATS request-reply latency tracking with p50/p95/p99 percentiles.

Follow the patterns in pkg/examples/metrics/.

Create histogram metric with appropriate buckets and a recording function.`,
				Language: session.LanguageGo,
			},
			// Add more requirements as needed
		},
	}
}

func (r *registry) Lookup(projectID, requirementCode string) (*Metadata, error) {
	key := fmt.Sprintf("%s:%s", projectID, requirementCode)
	meta, exists := r.metadata[key]
	if !exists {
		return nil, fmt.Errorf("requirement not found: %s", requirementCode)
	}
	return meta, nil
}
