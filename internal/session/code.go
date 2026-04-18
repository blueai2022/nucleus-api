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
		"--permission-mode", "auto",
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
	inputBuf.WriteString(request.Prompt)
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

	// Default to extracting code blocks from stdout
	code := extractCode(output)

	// If target file specified, try to generate a diff
	if request.TargetFile != "" {
		filePath := filepath.Join(s.WorkspaceRoot, request.TargetFile)
		if newContent, err := os.ReadFile(filePath); err == nil {
			newContentStr := string(newContent)

			// Only use diff if the file actually changed
			if originalContent != newContentStr {
				diff := generateUnifiedDiff(originalContent, newContentStr, request.TargetFile)
				if diff != "" {
					code = diff
					log.Debug().
						Str("target_file", request.TargetFile).
						Int("diff_length", len(code)).
						Msg("generated diff for modified file")
				} else {
					// File exists but no changes detected, return new content
					code = newContentStr
				}
			} else {
				// No changes, fall back to extractCode from stdout
				log.Debug().
					Str("target_file", request.TargetFile).
					Msg("file unchanged, using extracted code from output")
			}
		} else {
			log.Warn().
				Err(err).
				Str("target_file", request.TargetFile).
				Msg("failed to read target file, using extracted code from output")
		}
	}

	return &CodeGenerationResponse{
		Code:       code,
		RawOutput:  output,
		TargetFile: request.TargetFile,
	}, nil
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
func generateUnifiedDiff(original, modified, filename string) string {
	if original == modified {
		return "" // No changes
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(modified),
		FromFile: "a/" + filename,
		ToFile:   "b/" + filename,
		Context:  3, // Lines of context
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "" // Fallback
	}

	return result
}
