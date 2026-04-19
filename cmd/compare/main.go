package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file1> <file2>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s test_fixtures/projects/my-service/nats/metrics.go workspaces/test-001/nats/metrics.go\n", os.Args[0])
		os.Exit(1)
	}

	file1 := os.Args[1]
	file2 := os.Args[2]

	content1, err := os.ReadFile(file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file1, err)
		os.Exit(1)
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file2, err)
		os.Exit(1)
	}

	diff, err := generateUnifiedDiff(string(content1), string(content2), file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating diff: %v\n", err)
		os.Exit(1)
	}

	if diff == "" {
		fmt.Println("Files are identical")

		return
	}

	printHighlightedDiff(diff)
}

func printHighlightedDiff(diff string) {
	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			fmt.Println(yellow(line))
		case strings.HasPrefix(line, "@@"):
			fmt.Println(cyan(line))
		case strings.HasPrefix(line, "+"):
			fmt.Println(green(line))
		case strings.HasPrefix(line, "-"):
			fmt.Println(red(line))
		default:
			fmt.Println(line)
		}
	}
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
		return "", err
	}

	return result, nil
}
