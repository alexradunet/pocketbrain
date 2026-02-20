// Package workspace provides simple file operations within a root directory.
//
// File synchronisation is handled externally by Taildrive.
// This package only handles local file operations with path security.
package workspace

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SearchMode controls what fields are matched during SearchFiles.
type SearchMode string

const (
	SearchModeName    SearchMode = "name"
	SearchModeContent SearchMode = "content"
	SearchModeBoth    SearchMode = "both"
)

// WorkspaceFile describes a single entry returned by list/search operations.
type WorkspaceFile struct {
	Path        string
	Name        string
	Size        int64
	Modified    time.Time
	IsDirectory bool
}

// WorkspaceStats summarises aggregate information about the workspace.
type WorkspaceStats struct {
	TotalFiles   int
	TotalSize    int64
	LastModified *time.Time
}

// Workspace implements file operations scoped to a root directory.
type Workspace struct {
	rootPath string
	logger   *slog.Logger
}

// New creates a new Workspace. Call Initialize to create the directory.
func New(rootPath string, logger *slog.Logger) *Workspace {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		abs = rootPath
	}
	return &Workspace{rootPath: abs, logger: logger}
}

func (w *Workspace) log() *slog.Logger {
	if w.logger != nil {
		return w.logger
	}
	return slog.Default()
}

// Initialize creates the workspace root directory if it does not exist.
func (w *Workspace) Initialize() error {
	if err := os.MkdirAll(w.rootPath, 0o755); err != nil {
		return fmt.Errorf("workspace initialize: %w", err)
	}
	w.log().Info("workspace initialized", "path", w.rootPath)
	return nil
}

// Stop is a no-op. Taildrive handles sync externally.
func (w *Workspace) Stop() error { return nil }

// RootPath returns the absolute root path of the workspace.
func (w *Workspace) RootPath() string { return w.rootPath }

// ReadFile returns the content of a workspace-relative file, or ("", false)
// when the file does not exist or is unsafe to read.
func (w *Workspace) ReadFile(relativePath string) (string, bool) {
	filePath, ok := w.resolveExistingPath(relativePath, false)
	if !ok {
		return "", false
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// WriteFile writes content to a workspace-relative path, creating parent
// directories as needed. Returns false on any security or I/O failure.
func (w *Workspace) WriteFile(relativePath, content string) bool {
	filePath, ok := w.resolveWritablePath(relativePath)
	if !ok {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		w.log().Error("workspace write: mkdir failed", "error", err)
		return false
	}

	if !w.isWritablePathSafe(filePath) {
		return false
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		w.log().Error("workspace write failed", "error", err)
		return false
	}
	return true
}

// AppendToFile appends content to a workspace-relative file. If the file does
// not exist it is created. Returns false on failure.
func (w *Workspace) AppendToFile(relativePath, content string) bool {
	filePath, ok := w.resolveWritablePath(relativePath)
	if !ok {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		w.log().Error("workspace append: mkdir failed", "error", err)
		return false
	}

	if !w.isWritablePathSafe(filePath) {
		return false
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		w.log().Error("workspace append: open failed", "error", err)
		return false
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		w.log().Error("workspace append failed", "error", err)
		return false
	}
	return true
}

// ListFiles lists the direct children of a workspace-relative folder. Pass an
// empty string for the workspace root. Hidden entries and symlinks are skipped.
func (w *Workspace) ListFiles(folderPath string) ([]WorkspaceFile, error) {
	fullPath, ok := w.resolveExistingPath(folderPath, true)
	if !ok {
		return nil, nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, nil //nolint:nilerr // return empty on error
	}

	files := make([]WorkspaceFile, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		entryPath := filepath.Join(fullPath, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		files = append(files, WorkspaceFile{
			Path:        filepath.Join(folderPath, entry.Name()),
			Name:        entry.Name(),
			Size:        info.Size(),
			Modified:    info.ModTime(),
			IsDirectory: entry.IsDir(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDirectory != files[j].IsDirectory {
			return files[i].IsDirectory
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

// SearchFiles searches for files whose name and/or content matches query.
func (w *Workspace) SearchFiles(query, folder string, mode SearchMode) ([]WorkspaceFile, error) {
	allFiles, err := w.listFilesRecursive(folder)
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	searchMode := normalizeSearchMode(mode)

	if searchMode == SearchModeName {
		var out []WorkspaceFile
		for _, f := range allFiles {
			if strings.Contains(strings.ToLower(f.Name), lowerQuery) {
				out = append(out, f)
			}
		}
		return out, nil
	}

	var matched []WorkspaceFile
	for _, f := range allFiles {
		if f.IsDirectory {
			continue
		}

		nameMatch := strings.Contains(strings.ToLower(f.Name), lowerQuery)
		if searchMode == SearchModeBoth && nameMatch {
			matched = append(matched, f)
			continue
		}

		content, ok := w.ReadFile(f.Path)
		if ok && strings.Contains(strings.ToLower(content), lowerQuery) {
			matched = append(matched, f)
		}
	}

	return matched, nil
}

// MoveFile moves a file from fromPath to toPath (both workspace-relative).
func (w *Workspace) MoveFile(fromPath, toPath string) bool {
	source, ok := w.resolveExistingPath(fromPath, false)
	if !ok {
		return false
	}

	dest, ok := w.resolveWritablePath(toPath)
	if !ok {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		w.log().Error("workspace move: mkdir failed", "error", err)
		return false
	}

	if !w.isWritablePathSafe(dest) {
		return false
	}

	if err := os.Rename(source, dest); err != nil {
		w.log().Error("workspace move failed", "error", err)
		return false
	}
	return true
}

// GetStats returns aggregate statistics for all non-directory files.
func (w *Workspace) GetStats() (*WorkspaceStats, error) {
	files, err := w.listFilesRecursive("")
	if err != nil {
		return nil, err
	}

	var stats WorkspaceStats
	for _, f := range files {
		if !f.IsDirectory {
			stats.TotalSize += f.Size
			if stats.LastModified == nil || f.Modified.After(*stats.LastModified) {
				t := f.Modified
				stats.LastModified = &t
			}
		}
	}
	stats.TotalFiles = len(files)
	return &stats, nil
}

// ---------------------------------------------------------------------------
// Internal: recursive listing
// ---------------------------------------------------------------------------

func (w *Workspace) listFilesRecursive(folder string) ([]WorkspaceFile, error) {
	return w.walkFiles(folder)
}

func (w *Workspace) walkFiles(folder string) ([]WorkspaceFile, error) {
	startPath, ok := w.resolveExistingPath(folder, true)
	if !ok {
		return nil, nil
	}

	var all []WorkspaceFile
	err := filepath.WalkDir(startPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == startPath {
			return nil
		}

		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		rel, relErr := filepath.Rel(w.rootPath, path)
		if relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}

		all = append(all, WorkspaceFile{
			Path:        rel,
			Name:        d.Name(),
			Size:        info.Size(),
			Modified:    info.ModTime(),
			IsDirectory: d.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return all, nil
}

// ---------------------------------------------------------------------------
// Internal: path security
// ---------------------------------------------------------------------------

func (w *Workspace) resolvePath(inputPath string, allowRoot bool) (string, bool) {
	trimmed := strings.TrimSpace(inputPath)
	if !allowRoot && trimmed == "" {
		return "", false
	}

	resolved := filepath.Join(w.rootPath, trimmed)
	rel, err := filepath.Rel(w.rootPath, resolved)
	if err != nil {
		return "", false
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return resolved, true
}

func (w *Workspace) resolveExistingPath(inputPath string, allowRoot bool) (string, bool) {
	resolved, ok := w.resolvePath(inputPath, allowRoot)
	if !ok {
		return "", false
	}
	if !w.isExistingPathSafe(resolved) {
		return "", false
	}
	return resolved, true
}

func (w *Workspace) resolveWritablePath(inputPath string) (string, bool) {
	resolved, ok := w.resolvePath(inputPath, false)
	if !ok {
		return "", false
	}
	if !w.isWritablePathSafe(resolved) {
		return "", false
	}
	return resolved, true
}

func (w *Workspace) isExistingPathSafe(targetPath string) bool {
	if !w.hasNoSymlinkSegments(targetPath) {
		return false
	}

	rootReal, err := filepath.EvalSymlinks(w.rootPath)
	if err != nil {
		return false
	}
	targetReal, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return false
	}
	return isWithinRoot(rootReal, targetReal)
}

func (w *Workspace) isWritablePathSafe(targetPath string) bool {
	if !w.hasNoSymlinkSegments(targetPath) {
		return false
	}

	ancestor, ok := w.findNearestExistingAncestor(targetPath)
	if !ok {
		return false
	}

	rootReal, err := filepath.EvalSymlinks(w.rootPath)
	if err != nil {
		return false
	}

	ancestorReal, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return false
	}

	if !isWithinRoot(rootReal, ancestorReal) {
		return false
	}

	info, err := os.Lstat(targetPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		targetReal, err := filepath.EvalSymlinks(targetPath)
		if err != nil {
			return false
		}
		return isWithinRoot(rootReal, targetReal)
	}

	return true
}

func (w *Workspace) hasNoSymlinkSegments(targetPath string) bool {
	rel, err := filepath.Rel(w.rootPath, targetPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}

	segments := strings.Split(rel, string(filepath.Separator))
	current := w.rootPath

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		current = filepath.Join(current, seg)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}

	return true
}

func (w *Workspace) findNearestExistingAncestor(targetPath string) (string, bool) {
	current := targetPath
	for {
		_, err := os.Lstat(current)
		if err == nil {
			return current, true
		}
		if !os.IsNotExist(err) {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func isWithinRoot(rootPath, candidatePath string) bool {
	rel, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func normalizeSearchMode(mode SearchMode) SearchMode {
	switch mode {
	case SearchModeContent, SearchModeBoth, SearchModeName:
		return mode
	default:
		return SearchModeName
	}
}
