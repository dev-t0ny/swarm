package link

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
