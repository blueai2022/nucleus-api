package session

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	nucleusv1 "github.com/blueai2022/nucleus/pkg/nucleus/v1"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/rs/zerolog/log"
)

// ClaudeCodeSession represents a session using Claude Code CLI
type ClaudeCodeSession struct {
	ProjectID     string
	WorkspaceRoot string
}

// NewClaudeCodeSession creates a new Claude Code session
func NewClaudeCodeSession(projectID, workspaceRoot string) (*ClaudeCodeSession, error) {
	if err := verifyClaudeCode(); err != nil {
		return nil, fmt.Errorf("claude code not available: %w", err)
	}

	return &ClaudeCodeSession{
		ProjectID:     projectID,
		WorkspaceRoot: workspaceRoot,
	}, nil
}

// verifyClaudeCode checks if Claude Code CLI is available
func verifyClaudeCode() error {
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude command not found.")
	}
	return nil
}

// Generate generates code using Claude Code with workspace context
func (s *ClaudeCodeSession) Generate(
	ctx context.Context,
	request CodeGenerationRequest,
) (*CodeGenerationResponse, error) {
	log.Info().
		Str("project_id", s.ProjectID).
		Str("context_file", request.ContextFile).
		Msg("invoking claude code")

	originalFiles := s.snapshotWorkspace()

	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--model", "sonnet",
		"--add-dir", ".",
	}

	for _, dir := range request.ExampleDirs {
		args = append(args, "--add-dir", dir)
	}

	// Build minimal prompt with file reference, Claude tools read it
	fullPrompt := buildPromptWithFileReference(request)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = s.WorkspaceRoot

	var inputBuf bytes.Buffer
	inputBuf.WriteString(fullPrompt)
	cmd.Stdin = &inputBuf

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Error().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("claude code execution failed")
		return nil, fmt.Errorf("claude code failed: %w\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()

	log.Debug().
		Int("output_length", len(output)).
		Msg("received claude code response")

	response := &CodeGenerationResponse{
		TextResponse:  extractTextResponse(output),
		RawOutput:     output,
		ContextFile:   request.ContextFile,
		ModifiedFiles: make([]FileChange, 0),
		NewFiles:      make([]FileInfo, 0),
	}

	s.detectChanges(originalFiles, response)

	response.MainCodeChange = determineMainCodeChange(
		request.ContextFile,
		response.ModifiedFiles,
		response.NewFiles,
	)

	return response, nil
}

// snapshotWorkspace captures the current state of all files in the workspace
func (s *ClaudeCodeSession) snapshotWorkspace() map[string]string {
	snapshot := make(map[string]string)

	filepath.Walk(s.WorkspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden files and node_modules, etc.
		if strings.Contains(path, "/.") || strings.Contains(path, "/node_modules/") {
			return nil
		}

		relPath, err := filepath.Rel(s.WorkspaceRoot, path)
		if err != nil {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		snapshot[relPath] = string(content)
		return nil
	})

	return snapshot
}

// detectChanges compares before/after snapshots and populates the response
func (s *ClaudeCodeSession) detectChanges(
	originalFiles map[string]string,
	response *CodeGenerationResponse,
) {
	currentFiles := s.snapshotWorkspace()

	// Find modified files
	for filePath, originalContent := range originalFiles {
		if newContent, exists := currentFiles[filePath]; exists {
			if originalContent != newContent {
				diff, err := generateUnifiedDiff(originalContent, newContent, filePath)
				if err != nil {
					log.Error().Err(err).Str("file", filePath).Msg("failed to generate diff")
					continue
				}

				response.ModifiedFiles = append(response.ModifiedFiles, FileChange{
					Path:            filePath,
					OriginalContent: originalContent,
					NewContent:      newContent,
					Diff:            diff,
				})

				log.Debug().
					Str("file", filePath).
					Int("diff_length", len(diff)).
					Msg("detected modified file")
			}
		}
	}

	// Find new files
	for filePath, content := range currentFiles {
		if _, existed := originalFiles[filePath]; !existed {
			response.NewFiles = append(response.NewFiles, FileInfo{
				Path:    filePath,
				Content: content,
			})

			log.Debug().
				Str("file", filePath).
				Msg("detected new file")
		}
	}
}

// buildPromptWithFileReference creates a prompt referencing the file (like Copilot's pin)
func buildPromptWithFileReference(request CodeGenerationRequest) string {
	if request.ContextFile == "" {
		return request.Prompt
	}

	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("📌 Reference file: %s", request.ContextFile))

	// Add optional line range
	if request.StartLine > 0 && request.EndLine > 0 {
		prompt.WriteString(fmt.Sprintf(" (lines %d-%d)", request.StartLine, request.EndLine))
	}

	prompt.WriteString("\n\n")
	prompt.WriteString(request.Prompt)

	return prompt.String()
}

// determineMainCodeChange selects the primary code change based on precedence rules
func determineMainCodeChange(
	contextFile string,
	modifiedFiles []FileChange,
	newFiles []FileInfo,
) *CodeChange {
	// precedence rules: if context file was modified, use it
	if contextFile != "" {
		for _, fc := range modifiedFiles {
			if fc.Path == contextFile {
				return &CodeChange{
					Code:     fc.NewContent,
					FileName: fc.Path,
					FileType: FileTypeModified,
				}
			}
		}
	}

	// precedence rules: if only one file was modified, use it
	if len(modifiedFiles) == 1 && len(newFiles) == 0 {
		fc := modifiedFiles[0]
		return &CodeChange{
			Code:     fc.NewContent,
			FileName: fc.Path,
			FileType: FileTypeModified,
		}
	}

	// precedence rules: if only one file was created, use it
	if len(newFiles) == 1 && len(modifiedFiles) == 0 {
		nf := newFiles[0]
		return &CodeChange{
			Code:     nf.Content,
			FileName: nf.Path,
			FileType: FileTypeNew,
		}
	}

	// precedence rules: use the file with the most content (largest change)
	var largest *CodeChange
	maxLength := 0

	for _, fc := range modifiedFiles {
		if len(fc.NewContent) > maxLength {
			maxLength = len(fc.NewContent)
			largest = &CodeChange{
				Code:     fc.NewContent,
				FileName: fc.Path,
				FileType: FileTypeModified,
			}
		}
	}

	for _, nf := range newFiles {
		if len(nf.Content) > maxLength {
			maxLength = len(nf.Content)
			largest = &CodeChange{
				Code:     nf.Content,
				FileName: nf.Path,
				FileType: FileTypeNew,
			}
		}
	}

	return largest
}

// CodeGenerationRequest represents a request to generate code
type CodeGenerationRequest struct {
	Prompt      string
	ContextFile string // Optional: file to reference as context (like Copilot's pinned file)
	StartLine   int    // Optional: start of highlighted range (1-indexed)
	EndLine     int    // Optional: end of highlighted range (1-indexed)
	ExampleDirs []string
}

// CodeGenerationResponse represents Claude Code's comprehensive response
type CodeGenerationResponse struct {
	TextResponse   string       // TODO: will be removed - use RawOutput
	RawOutput      string       // Complete stdout from Claude
	ContextFile    string       // The file that was used as context
	ModifiedFiles  []FileChange // Files that were changed
	NewFiles       []FileInfo   // Files that were created
	MainCodeChange *CodeChange  // The primary code change based on precedence rules
}

// CodeChange represents the main code change
type CodeChange struct {
	Code     string // Full code content after Claude modifications
	FileName string // File name for the code file
	// TODO: add deleted file type
	FileType FileType // Whether this is a new or modified file
}

// FileType indicates whether a file is new or modified
type FileType string

const (
	FileTypeNew      FileType = "new"
	FileTypeModified FileType = "modified"
)

func (ft FileType) ToProto() nucleusv1.FileType {
	switch ft {
	case FileTypeNew:
		return nucleusv1.FileType_FILE_TYPE_NEW
	case FileTypeModified:
		return nucleusv1.FileType_FILE_TYPE_MODIFIED
	default:
		return nucleusv1.FileType_FILE_TYPE_UNSPECIFIED
	}
}

// FileChange represents a modified file with diff
type FileChange struct {
	Path            string // Relative path from workspace root
	OriginalContent string
	NewContent      string
	Diff            string // Unified diff format
}

// FileInfo represents a new or complete file
type FileInfo struct {
	Path    string // Relative path from workspace root
	Content string // Complete file content
}

// extractTextResponse extracts Claude's text explanation from the output
func extractTextResponse(output string) string {
	var textParts []string
	scanner := bufio.NewScanner(strings.NewReader(output))

	inCodeBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if !inCodeBlock && strings.TrimSpace(line) != "" {
			textParts = append(textParts, line)
		}
	}

	return strings.Join(textParts, "\n")
}

// extractCode extracts code blocks from Claude's response
func extractCode(output string) string {
	var codeBlocks []string
	scanner := bufio.NewScanner(strings.NewReader(output))

	inCodeBlock := false
	var currentBlock strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				codeBlocks = append(codeBlocks, currentBlock.String())
				currentBlock.Reset()
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			currentBlock.WriteString(line)
			currentBlock.WriteString("\n")
		}
	}

	if len(codeBlocks) > 0 {
		return codeBlocks[len(codeBlocks)-1]
	}

	return output
}

// generateUnifiedDiff creates a unified diff between old and new content
func generateUnifiedDiff(original, modified, filename string) (string, error) {
	if original == modified {
		return "", nil
	}

	const linesOfContext = 3

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(modified),
		FromFile: "a/" + filename,
		ToFile:   "b/" + filename,
		Context:  linesOfContext,
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", fmt.Errorf("failed to generate unified diff: %w", err)
	}

	return result, nil
}
