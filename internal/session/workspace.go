package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// Language represents supported programming languages
type Language string

const (
	LanguageGo     Language = "go"
	LanguageJava   Language = "java"
	LanguagePython Language = "python"
	LanguageNodeJS Language = "nodejs"
)

// ProjectType represents the type of project
type ProjectType string

const (
	ProjectTypeBackend  ProjectType = "backend"
	ProjectTypeFrontend ProjectType = "frontend"
)

// WorkspaceConfig contains parameters for workspace composition
type WorkspaceConfig struct {
	ProjectID            string
	Language             Language
	Type                 ProjectType
	MainProjectPath      string   // Path to the main project codebase
	TemplateRequirements []string // e.g., ["connect", "metrics"] - requirements needing template examples
}

// WorkspaceComposer handles dynamic workspace composition
type WorkspaceComposer interface {
	Compose(config WorkspaceConfig) (string, error)
	Destroy(projectID string) error
}

// workspaceComposer is the concrete implementation of WorkspaceComposer
type workspaceComposer struct {
	workspaceRoot string
	templateRoot  string
}

// NewWorkspaceComposer creates a new workspace composer
func NewWorkspaceComposer(workspaceRoot, templateRoot string) WorkspaceComposer {
	return &workspaceComposer{
		workspaceRoot: workspaceRoot,
		templateRoot:  templateRoot,
	}
}

// Compose creates a workspace by combining main project code with template examples
// Returns the absolute path to the created workspace
func (wc *workspaceComposer) Compose(config WorkspaceConfig) (string, error) {
	if err := wc.validateConfig(config); err != nil {
		return "", fmt.Errorf("invalid workspace config: %w", err)
	}

	workspacePath := filepath.Join(wc.workspaceRoot, config.ProjectID)

	// Copy main project codebase to workspace root
	if err := wc.copyMainProject(config.MainProjectPath, workspacePath); err != nil {
		os.RemoveAll(workspacePath)
		return "", fmt.Errorf("failed to copy main project: %w", err)
	}

	log.Info().
		Str("project_id", config.ProjectID).
		Str("main_project", config.MainProjectPath).
		Str("workspace", workspacePath).
		Msg("copied main project to workspace")

	// Merge template examples into workspace
	wc.mergeTemplateExamples(config, workspacePath)

	return workspacePath, nil
}

// mergeTemplateExamples merges PR-approved template code into the workspace
func (wc *workspaceComposer) mergeTemplateExamples(config WorkspaceConfig, workspacePath string) {
	if len(config.TemplateRequirements) == 0 {
		log.Info().
			Str("project_id", config.ProjectID).
			Msg("no template examples requested")
		return
	}

	examplesPath := wc.examplesPath(workspacePath, config.Language)
	copiedCount := wc.copyTemplateExamples(config, examplesPath)

	log.Info().
		Str("project_id", config.ProjectID).
		Int("templates_copied", copiedCount).
		Int("templates_requested", len(config.TemplateRequirements)).
		Msg("merged template examples into workspace")
}

// examplesPath returns the language-specific examples directory path within the workspace
func (wc *workspaceComposer) examplesPath(workspacePath string, lang Language) string {
	switch lang {
	case LanguageGo:
		return filepath.Join(workspacePath, "pkg", "examples")
	case LanguageJava:
		return filepath.Join(workspacePath, "src", "examples")
	case LanguagePython:
		return filepath.Join(workspacePath, "examples")
	case LanguageNodeJS:
		return filepath.Join(workspacePath, "examples")
	default:
		return filepath.Join(workspacePath, "examples")
	}
}

// copyMainProject copies the main project codebase to the workspace
func (wc *workspaceComposer) copyMainProject(mainProjectPath, workspacePath string) error {
	// Verify main project exists
	if _, err := os.Stat(mainProjectPath); os.IsNotExist(err) {
		return fmt.Errorf("main project not found: %s", mainProjectPath)
	}

	// Create workspace directory
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	return wc.copyDirectory(mainProjectPath, workspacePath)
}

// copyTemplateExamples copies template code examples for the specified requirements
// Returns the number of templates successfully copied
func (wc *workspaceComposer) copyTemplateExamples(config WorkspaceConfig, examplesPath string) int {
	copiedCount := 0

	for _, requirement := range config.TemplateRequirements {
		templatePath := filepath.Join(
			wc.templateRoot,
			string(config.Type),
			string(config.Language),
			wc.templateSubdir(config.Language),
			requirement,
		)

		destPath := filepath.Join(examplesPath, requirement)

		// Soft check: no template doesn't exist, warn but continue
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			log.Warn().
				Str("requirement", requirement).
				Str("template_path", templatePath).
				Msg("template not found, skipping")

			continue
		}

		// Copy requirement template to examples/[requirement]
		if err := wc.copyDirectory(templatePath, destPath); err != nil {
			log.Warn().
				Str("requirement", requirement).
				Err(err).
				Msg("failed to copy template")
			continue
		}

		log.Debug().
			Str("requirement", requirement).
			Str("dest", destPath).
			Msg("copied template example")

		copiedCount++
	}

	return copiedCount
}

// validateConfig validates the workspace configuration
func (wc *workspaceComposer) validateConfig(config WorkspaceConfig) error {
	if config.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	if config.MainProjectPath == "" {
		return fmt.Errorf("main_project_path is required")
	}

	// Soft check: warn if language is not explicitly supported
	knownLanguages := map[Language]bool{
		LanguageGo:     true,
		LanguageJava:   true,
		LanguagePython: true,
		LanguageNodeJS: true,
	}
	if !knownLanguages[config.Language] {
		log.Warn().
			Str("language", string(config.Language)).
			Str("project_id", config.ProjectID).
			Msg("unknown language specified, will use default paths")
	}

	return nil
}

// templateSubdir returns the template subdirectory for the language
func (wc *workspaceComposer) templateSubdir(lang Language) string {
	switch lang {
	case LanguageGo:
		return "pkg"
	case LanguageJava:
		return filepath.Join("src", "examples")
	case LanguagePython:
		return "examples"
	case LanguageNodeJS:
		return "examples"
	default:
		return "examples"
	}
}

// copyDirectory recursively copies a directory from src to dest
func (wc *workspaceComposer) copyDirectory(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source
		relativePath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Construct destination path
		destPath := filepath.Join(dest, relativePath)

		// If directory, create it
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// If file, copy it
		return wc.copyFile(path, destPath, info.Mode())
	})
}

// copyFile copies a single file from src to dest with the given permissions
func (wc *workspaceComposer) copyFile(src, dest string, mode os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Set file permissions to match source
	if err := os.Chmod(dest, mode); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// Destroy removes a workspace directory
func (wc *workspaceComposer) Destroy(projectID string) error {
	workspacePath := filepath.Join(wc.workspaceRoot, projectID)
	if err := os.RemoveAll(workspacePath); err != nil {
		return fmt.Errorf("failed to destroy workspace: %w", err)
	}

	log.Info().
		Str("project_id", projectID).
		Str("workspace", workspacePath).
		Msg("destroyed workspace")

	return nil
}
