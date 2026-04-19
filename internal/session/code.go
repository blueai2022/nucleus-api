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
		return fmt.Errorf("claude command not found. Install: npm install -g @anthropic-ai/claude-code")
	}
	return nil
}

// Generate generates code using Claude Code with workspace context
func (s *ClaudeCodeSession) Generate(ctx context.Context, request CodeGenerationRequest) (*CodeGenerationResponse, error) {
	log.Info().
		Str("project_id", s.ProjectID).
		Str("target_file", request.TargetFile).
		Msg("invoking claude code")

	// Read the target file BEFORE Claude runs (if it exists)
	var originalContent string
	if request.TargetFile != "" {
		filePath := filepath.Join(s.WorkspaceRoot, request.TargetFile)
		if fileContent, err := os.ReadFile(filePath); err == nil {
			originalContent = string(fileContent)
		}
	}

	args := []string{
		"-p",
		"--dangerously-skip-permissions",
		"--model", "sonnet",
		"--add-dir", ".",
	}

	if len(request.ExampleDirs) > 0 {
		for _, dir := range request.ExampleDirs {
			args = append(args, "--add-dir", dir)
		}
	}

	fullPrompt := request.Prompt
	if request.TargetFile != "" {
		fullPrompt = fmt.Sprintf("Working on file: %s\n\n%s", request.TargetFile, request.Prompt)
	}

	args = append(args, fullPrompt)

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

	// Generate result: diff if file changed, otherwise extracted code from stdout
	code := s.generateResult(request.TargetFile, originalContent, output)

	return &CodeGenerationResponse{
		Code:       code,
		RawOutput:  output,
		TargetFile: request.TargetFile,
	}, nil
}

// generateResult determines what to return: a diff if the file was modified,
// the new file content if created, or extracted code blocks from stdout.
func (s *ClaudeCodeSession) generateResult(targetFile, originalContent, stdout string) string {
	if targetFile == "" {
		return extractCode(stdout)
	}

	filePath := filepath.Join(s.WorkspaceRoot, targetFile)
	newContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Warn().
			Err(err).
			Str("target_file", targetFile).
			Msg("target file not found after execution, using extracted code from output")
		return extractCode(stdout)
	}

	newContentStr := string(newContent)

	// File unchanged - return extracted code from stdout
	if originalContent == newContentStr {
		log.Debug().
			Str("target_file", targetFile).
			Msg("file unchanged, using extracted code from output")

		return extractCode(stdout)
	}

	diff, err := generateUnifiedDiff(originalContent, newContentStr, targetFile)
	if err != nil {
		log.Error().
			Err(err).
			Str("target_file", targetFile).
			Msg("failed to generate diff, returning new content")

		return newContentStr
	}

	if diff != "" {
		log.Debug().
			Str("target_file", targetFile).
			Int("diff_length", len(diff)).
			Msg("generated diff for modified file")

		return diff
	}

	// Diff generation returned "" due to some error, use fallback
	return newContentStr
}

// CodeGenerationRequest represents a request to generate code
type CodeGenerationRequest struct {
	Prompt      string
	TargetFile  string // Optional: file to pin as context (like Copilot)
	StartLine   int    // Optional: start of highlighted range (1-indexed)
	EndLine     int    // Optional: end of highlighted range (1-indexed)
	ExampleDirs []string
}

// CodeGenerationResponse represents Claude Code's response
type CodeGenerationResponse struct {
	Code       string
	RawOutput  string
	TargetFile string
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
