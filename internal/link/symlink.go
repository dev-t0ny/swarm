package link

import (
	"fmt"
	"os"
	"path/filepath"
)

// Linker handles symlinking config files into worktrees.
type Linker struct {
	SourceDir string
	Files     []string
}

// NewLinker creates a new symlink linker.
func NewLinker(sourceDir string, files []string) *Linker {
	return &Linker{
		SourceDir: sourceDir,
		Files:     files,
	}
}

// LinkResult records the outcome of a symlink operation.
type LinkResult struct {
	File    string
	Created bool
	Skipped bool
	Error   error
}

// LinkTo creates symlinks for all configured files into the target directory.
// Files that don't exist in the source directory are silently skipped.
// Existing files in the target directory are removed before linking.
func (l *Linker) LinkTo(targetDir string) []LinkResult {
	var results []LinkResult

	for _, file := range l.Files {
		result := LinkResult{File: file}
		sourcePath := filepath.Join(l.SourceDir, file)
		targetPath := filepath.Join(targetDir, file)

		// Check if source file exists
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			result.Skipped = true
			results = append(results, result)
			continue
		}

		// Ensure target parent directory exists (for nested paths like credentials/key.json)
		targetParent := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetParent, 0755); err != nil {
			result.Error = fmt.Errorf("create parent dir for %s: %w", file, err)
			results = append(results, result)
			continue
		}

		// Remove existing file/link at target
		_ = os.Remove(targetPath)

		// Create the symlink (absolute path)
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			result.Error = fmt.Errorf("resolve absolute path for %s: %w", file, err)
			results = append(results, result)
			continue
		}

		if err := os.Symlink(absSource, targetPath); err != nil {
			result.Error = fmt.Errorf("symlink %s: %w", file, err)
			results = append(results, result)
			continue
		}

		result.Created = true
		results = append(results, result)
	}

	return results
}

// UnlinkFrom removes symlinks for all configured files from the target directory.
func (l *Linker) UnlinkFrom(targetDir string) {
	for _, file := range l.Files {
		targetPath := filepath.Join(targetDir, file)
		info, err := os.Lstat(targetPath)
		if err != nil {
			continue
		}
		// Only remove if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(targetPath)
		}
	}
}
