package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/toolrunner"
)

const (
	// TODO: move to config
	model = "claude-sonnet-4-5-20250929"
)

// Tool input structs with jsonschema tags for auto-schema generation
type ReadFileInput struct {
	Path string `json:"path" jsonschema:"required,description=Relative path to the file"`
}

type ListDirectoryInput struct {
	Path string `json:"path" jsonschema:"required,description=Directory path relative to workspace root"`
}

type SearchCodeInput struct {
	Pattern   string `json:"pattern" jsonschema:"required,description=Regex pattern to search for"`
	Directory string `json:"directory,omitempty" jsonschema:"description=Optional directory to search within"`
}

type FindFilesInput struct {
	Glob string `json:"glob" jsonschema:"required,description=Glob pattern for file matching"`
}

// NewSession creates a new chat session with workspace tools
func NewSession(client anthropic.Client, projectID, workspaceRoot string) (*Session, error) {
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	s := &Session{
		ProjectID:     projectID,
		WorkspaceRoot: workspaceRoot,
		client:        client,
		messages:      []anthropic.BetaMessageParam{},
	}

	if err := s.initializeTools(); err != nil {
		return nil, fmt.Errorf("failed to initialize tools: %w", err)
	}

	return s, nil
}

// initializeTools creates all workspace tools with their handlers
func (s *Session) initializeTools() error {
	readFileTool, err := s.createReadFileTool()
	if err != nil {
		return err
	}

	listDirTool, err := s.createListDirectoryTool()
	if err != nil {
		return err
	}

	searchTool, err := s.createSearchCodeTool()
	if err != nil {
		return err
	}

	findTool, err := s.createFindFilesTool()
	if err != nil {
		return err
	}

	s.tools = []anthropic.BetaTool{readFileTool, listDirTool, searchTool, findTool}
	return nil
}

func (s *Session) createReadFileTool() (anthropic.BetaTool, error) {
	return toolrunner.NewBetaToolFromJSONSchema(
		"read_file",
		"Read the full contents of a file given its path relative to the workspace root.",
		func(ctx context.Context, input ReadFileInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			content, err := s.readFile(input.Path)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{
					OfText: &anthropic.BetaTextBlockParam{
						Text: fmt.Sprintf("Error reading file: %v", err),
					},
				}, nil
			}
			return anthropic.BetaToolResultBlockParamContentUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Text: content,
				},
			}, nil
		},
	)
}

func (s *Session) createListDirectoryTool() (anthropic.BetaTool, error) {
	return toolrunner.NewBetaToolFromJSONSchema(
		"list_directory",
		"List files and subdirectories at a given path.",
		func(ctx context.Context, input ListDirectoryInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			listing, err := s.listDirectory(input.Path)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{
					OfText: &anthropic.BetaTextBlockParam{
						Text: fmt.Sprintf("Error listing directory: %v", err),
					},
				}, nil
			}
			return anthropic.BetaToolResultBlockParamContentUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Text: listing,
				},
			}, nil
		},
	)
}

func (s *Session) createSearchCodeTool() (anthropic.BetaTool, error) {
	return toolrunner.NewBetaToolFromJSONSchema(
		"search_code",
		"Search for a regex pattern across files in the workspace.",
		func(ctx context.Context, input SearchCodeInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			results, err := s.searchCode(input.Pattern, input.Directory)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{
					OfText: &anthropic.BetaTextBlockParam{
						Text: fmt.Sprintf("Error searching code: %v", err),
					},
				}, nil
			}
			return anthropic.BetaToolResultBlockParamContentUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Text: results,
				},
			}, nil
		},
	)
}

func (s *Session) createFindFilesTool() (anthropic.BetaTool, error) {
	return toolrunner.NewBetaToolFromJSONSchema(
		"find_files",
		"Find files matching a glob pattern under the workspace root.",
		func(ctx context.Context, input FindFilesInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			files, err := s.findFiles(input.Glob)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{
					OfText: &anthropic.BetaTextBlockParam{
						Text: fmt.Sprintf("Error finding files: %v", err),
					},
				}, nil
			}
			return anthropic.BetaToolResultBlockParamContentUnion{
				OfText: &anthropic.BetaTextBlockParam{
					Text: files,
				},
			}, nil
		},
	)
}

// Chat sends a message and runs the agentic loop to completion
func (s *Session) Chat(ctx context.Context, userMsg string) (string, error) {
	s.messages = append(s.messages,
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(userMsg)),
	)

	var system []anthropic.BetaTextBlockParam
	if len(s.messages) == 1 {
		system = []anthropic.BetaTextBlockParam{
			{
				Text: s.systemPrompt(),
			},
		}
	}

	runner := s.client.Beta.Messages.NewToolRunner(s.tools,
		anthropic.BetaToolRunnerParams{
			BetaMessageNewParams: anthropic.BetaMessageNewParams{
				Model:     model,
				MaxTokens: 8192,
				Messages:  s.messages,
				System:    system,
			},
		},
	)

	// Note: RunToCompletion automatically handles tool loop
	message, err := runner.RunToCompletion(ctx)
	if err != nil {
		return "", fmt.Errorf("claude API error: %w", err)
	}

	s.messages = runner.Messages()

	var responseText strings.Builder

	for _, block := range message.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaTextBlock:
			responseText.WriteString(b.Text)
		}
	}

	return responseText.String(), nil
}

func (s *Session) readFile(relPath string) (string, error) {
	// Note:prevent path traversal - security first
	fullPath := filepath.Join(s.WorkspaceRoot, relPath)
	cleanPath := filepath.Clean(fullPath)

	if !strings.HasPrefix(cleanPath, filepath.Clean(s.WorkspaceRoot)) {
		return "", fmt.Errorf("path traversal attempt blocked")
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (s *Session) listDirectory(relPath string) (string, error) {
	// Note:prevent path traversal - security first
	fullPath := filepath.Join(s.WorkspaceRoot, relPath)
	cleanPath := filepath.Clean(fullPath)

	if !strings.HasPrefix(cleanPath, filepath.Clean(s.WorkspaceRoot)) {
		return "", fmt.Errorf("path traversal attempt blocked")
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(entry.Name() + "/\n")
		} else {
			info, _ := entry.Info()
			result.WriteString(fmt.Sprintf("%s (%d bytes)\n", entry.Name(), info.Size()))
		}
	}

	return result.String(), nil
}

func (s *Session) searchCode(pattern, directory string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	searchRoot := s.WorkspaceRoot
	if directory != "" {
		searchRoot = filepath.Join(s.WorkspaceRoot, directory)
		cleanPath := filepath.Clean(searchRoot)
		if !strings.HasPrefix(cleanPath, filepath.Clean(s.WorkspaceRoot)) {
			return "", fmt.Errorf("path traversal attempt blocked")
		}
	}

	var results []string
	err = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // TODO: Fail-fast here, skipping files with errors for now
		}
		if info.IsDir() {
			return nil
		}

		// Non-binary files only
		if !isTextFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			if re.MatchString(line) {
				relPath, _ := filepath.Rel(s.WorkspaceRoot, path)
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum+1, line))
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "No matches found", nil
	}

	return strings.Join(results, "\n"), nil
}

func (s *Session) findFiles(globPattern string) (string, error) {
	pattern := filepath.Join(s.WorkspaceRoot, globPattern)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "No files found", nil
	}

	var results []string
	for _, match := range matches {
		relPath, _ := filepath.Rel(s.WorkspaceRoot, match)
		results = append(results, relPath)
	}

	return strings.Join(results, "\n"), nil
}

func (s *Session) systemPrompt() string {
	return fmt.Sprintf(`You are an expert software engineer assistant integrated into a development platform.
You have access to the developer's local workspace at: %s

Use the provided tools to explore files, read source code, and understand the project structure
before generating implementation. Always read relevant files before writing code. Follow existing
conventions, naming patterns, and architecture found in the codebase.

When you generate code, be thorough and complete. Consider edge cases, error handling, and
integration with existing code patterns.`, s.WorkspaceRoot)
}

// isTextFile is a simple heuristic to determine if a file is text-based based on its extension.
//
// TODO: consider improving this by checking file headers or using a library for better accuracy.
func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".py": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".txt": true, ".md": true, ".json": true, ".yaml": true,
		".yml": true, ".xml": true, ".html": true, ".css": true,
		".sh": true, ".proto": true, ".sql": true,
	}

	return textExts[ext]
}

// manager manages multiple chat sessions
type manager struct {
	client        anthropic.Client
	workspaceRoot string
	sessions      map[string]*Session
}

// NewManager creates a new session manager
func NewManager(client anthropic.Client, workspaceRoot string) Manager {
	return &manager{
		client:        client,
		workspaceRoot: workspaceRoot,
		sessions:      make(map[string]*Session),
	}
}

// Create creates a new chat session for a project
func (m *manager) Create(ctx context.Context, projectID, requirementCode string) (*Session, error) {
	sessionID := fmt.Sprintf("%s-%s", projectID, requirementCode)
	workspaceDir := filepath.Join(m.workspaceRoot, sessionID)

	session, err := NewSession(m.client, projectID, workspaceDir)
	if err != nil {
		return nil, err
	}

	m.sessions[sessionID] = session
	return session, nil
}

// Session retrieves an existing session
func (m *manager) Session(projectID, requirementCode string) (*Session, error) {
	sessionID := fmt.Sprintf("%s-%s", projectID, requirementCode)
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// Close cleans up a session
func (m *manager) Close(projectID, requirementCode string) error {
	sessionID := fmt.Sprintf("%s-%s", projectID, requirementCode)
	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Clean up workspace
	if err := os.RemoveAll(session.WorkspaceRoot); err != nil {
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}

	delete(m.sessions, sessionID)
	return nil
}

// Streaming support for incremental responses
type StreamHandler func(text string) error

// ChatStreaming sends a message and streams responses back
func (s *Session) ChatStreaming(ctx context.Context, userMsg string, handler StreamHandler) error {
	s.messages = append(s.messages,
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(userMsg)),
	)

	var system []anthropic.BetaTextBlockParam
	if len(s.messages) == 1 {
		system = []anthropic.BetaTextBlockParam{{Text: s.systemPrompt()}}
	}

	runner := s.client.Beta.Messages.NewToolRunnerStreaming(s.tools,
		anthropic.BetaToolRunnerParams{
			BetaMessageNewParams: anthropic.BetaMessageNewParams{
				Model:     model,
				MaxTokens: 8192,
				Messages:  s.messages,
				System:    system,
			},
		},
	)

	// Stream events
	for eventsIterator := range runner.AllStreaming(ctx) {
		for event, err := range eventsIterator {
			if err != nil {
				return fmt.Errorf("streaming error: %w", err)
			}

			switch e := event.AsAny().(type) {
			case anthropic.BetaRawContentBlockDeltaEvent:
				switch delta := e.Delta.AsAny().(type) {
				case anthropic.BetaTextDelta:
					if err := handler(delta.Text); err != nil {
						return err
					}
				}
			}
		}
	}

	// Update history
	s.messages = runner.Messages()
	return nil
}
