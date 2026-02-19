// Package vault provides high-level vault file operations for PocketBrain.
//
// File synchronisation is handled externally by Taildrive.
// This service only handles local file operations.
package vault

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/config"
)

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

// VaultFile describes a single entry returned by list/search operations.
type VaultFile struct {
	Path        string
	Name        string
	Size        int64
	Modified    time.Time
	IsDirectory bool
}

// VaultSearchMode controls what fields are matched during SearchFiles.
type VaultSearchMode string

const (
	SearchModeName    VaultSearchMode = "name"
	SearchModeContent VaultSearchMode = "content"
	SearchModeBoth    VaultSearchMode = "both"
)

// VaultStats summarises aggregate information about the vault.
type VaultStats struct {
	TotalFiles   int
	TotalSize    int64
	LastModified *time.Time
}

// ObsidianConfigSummary contains a human-readable digest of the .obsidian
// configuration files found inside the vault.
type ObsidianConfigSummary struct {
	ObsidianConfigFound bool
	DailyNotes          ObsidianDailyNotes
	NewNotes            ObsidianNewNotes
	Attachments         ObsidianAttachments
	Links               ObsidianLinks
	Templates           ObsidianTemplates
	Warnings            []string
}

// ObsidianDailyNotes holds daily-notes plugin settings.
type ObsidianDailyNotes struct {
	Folder        string
	Format        string
	TemplateFile  string
	PluginEnabled bool
}

// ObsidianNewNotes holds new-note creation settings.
type ObsidianNewNotes struct {
	Location string // "current" | "folder" | "root" | "unknown"
	Folder   string
}

// ObsidianAttachments holds attachment folder settings.
type ObsidianAttachments struct {
	Folder string
}

// ObsidianLinks holds link-style settings.
type ObsidianLinks struct {
	Style string // "wikilink" | "markdown"
}

// ObsidianTemplates holds template folder settings.
type ObsidianTemplates struct {
	Folder string
}

// ObsidianConfigState is ObsidianConfigSummary plus cache metadata.
type ObsidianConfigState struct {
	Summary     ObsidianConfigSummary
	Fingerprint string
	CacheHit    bool
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

type dailyNoteSettings struct {
	folder       string
	format       string
	templateFile string // empty when absent
}

type obsidianConfigCache struct {
	fingerprint string
	summary     ObsidianConfigSummary
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Options configures a VaultService instance.
type Options struct {
	VaultPath       string
	DailyNoteFormat string
	Folders         config.VaultFolders
	Logger          *slog.Logger
}

// Service implements high-level vault operations.
type Service struct {
	opts        Options
	vaultRoot   string // resolved absolute path
	configCache *obsidianConfigCache
}

// New creates a new Service. Call Initialize to create the vault directory.
func New(opts Options) *Service {
	abs, err := filepath.Abs(opts.VaultPath)
	if err != nil {
		abs = opts.VaultPath
	}
	return &Service{
		opts:      opts,
		vaultRoot: abs,
	}
}

// log returns the configured logger or the default slog logger.
func (s *Service) log() *slog.Logger {
	if s.opts.Logger != nil {
		return s.opts.Logger
	}
	return slog.Default()
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Initialize creates the vault root directory if it does not exist.
func (s *Service) Initialize() error {
	if err := os.MkdirAll(s.opts.VaultPath, 0o755); err != nil {
		return fmt.Errorf("vault initialize: %w", err)
	}
	s.log().Info("vault initialized")
	return nil
}

// Stop is a no-op. Taildrive handles sync externally.
func (s *Service) Stop() {}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

// ReadFile returns the content of a vault-relative file, or ("", false) when
// the file does not exist or is unsafe to read.
func (s *Service) ReadFile(relativePath string) (string, bool) {
	filePath, ok := s.resolveExistingPathWithinVault(relativePath, false)
	if !ok {
		return "", false
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// WriteFile writes content to a vault-relative path, creating parent directories
// as needed. Returns false on any security or I/O failure.
func (s *Service) WriteFile(relativePath, content string) bool {
	filePath, ok := s.resolveWritablePathWithinVault(relativePath)
	if !ok {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		s.log().Error("vault write: mkdir failed", "error", err)
		return false
	}

	// Re-check after mkdir in case a symlink appeared between validation and
	// directory creation.
	if !s.isWritablePathSafe(filePath) {
		return false
	}

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		s.log().Error("vault write failed", "error", err)
		return false
	}
	return true
}

// AppendToFile appends content to a vault-relative file. If the file does not
// exist it is created. Returns false on failure.
func (s *Service) AppendToFile(relativePath, content string) bool {
	existing, ok := s.ReadFile(relativePath)
	if ok {
		return s.WriteFile(relativePath, existing+content)
	}
	return s.WriteFile(relativePath, content)
}

// ListFiles lists the direct children of a vault-relative folder. Pass an
// empty string for the vault root. Hidden entries and symlinks are skipped.
func (s *Service) ListFiles(folderPath string) ([]VaultFile, error) {
	fullPath, ok := s.resolveExistingPathWithinVault(folderPath, true)
	if !ok {
		return nil, nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, nil //nolint:nilerr // match TS behaviour: return empty on error
	}

	files := make([]VaultFile, 0, len(entries))
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

		files = append(files, VaultFile{
			Path:        filepath.Join(folderPath, entry.Name()),
			Name:        entry.Name(),
			Size:        info.Size(),
			Modified:    info.ModTime(),
			IsDirectory: entry.IsDir(),
		})
	}

	// Directories first, then alphabetical within each group.
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDirectory != files[j].IsDirectory {
			return files[i].IsDirectory
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

// SearchFiles searches for files whose name and/or content matches query.
// mode controls which fields are examined ("name", "content", or "both").
func (s *Service) SearchFiles(query, folder string, mode VaultSearchMode) ([]VaultFile, error) {
	allFiles, err := s.listFilesRecursive(folder)
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	searchMode := normalizeSearchMode(mode)

	if searchMode == SearchModeName {
		var out []VaultFile
		for _, f := range allFiles {
			if strings.Contains(strings.ToLower(f.Name), lowerQuery) {
				out = append(out, f)
			}
		}
		return out, nil
	}

	var matched []VaultFile
	for _, f := range allFiles {
		if f.IsDirectory {
			continue
		}

		nameMatch := strings.Contains(strings.ToLower(f.Name), lowerQuery)
		if searchMode == SearchModeBoth && nameMatch {
			matched = append(matched, f)
			continue
		}

		content, ok := s.ReadFile(f.Path)
		if ok && strings.Contains(strings.ToLower(content), lowerQuery) {
			matched = append(matched, f)
		}
	}

	return matched, nil
}

// FindBacklinks returns all files that contain a wiki-link whose normalised
// target equals the normalised form of target.
func (s *Service) FindBacklinks(target, folder string) ([]VaultFile, error) {
	normalizedTarget := NormalizeWikiLinkTarget(target)
	allFiles, err := s.listFilesRecursive(folder)
	if err != nil {
		return nil, err
	}

	var matches []VaultFile
	for _, f := range allFiles {
		if f.IsDirectory {
			continue
		}

		content, ok := s.ReadFile(f.Path)
		if !ok {
			continue
		}

		links := ParseWikiLinks(content)
		for _, link := range links {
			if link.NormalizedTarget == normalizedTarget {
				matches = append(matches, f)
				break
			}
		}
	}

	return matches, nil
}

// SearchByTag returns all files that contain a given #tag.
func (s *Service) SearchByTag(tag, folder string) ([]VaultFile, error) {
	normalizedTag := normalizeTag(tag)
	allFiles, err := s.listFilesRecursive(folder)
	if err != nil {
		return nil, err
	}

	var matches []VaultFile
	for _, f := range allFiles {
		if f.IsDirectory {
			continue
		}

		content, ok := s.ReadFile(f.Path)
		if !ok {
			continue
		}

		tags := ExtractMarkdownTags(content)
		for _, t := range tags {
			if t == normalizedTag {
				matches = append(matches, f)
				break
			}
		}
	}

	return matches, nil
}

// GetDailyNotePath returns today's daily note path using the configured daily
// folder and a YYYY-MM-DD filename. This is a fast, synchronous version that
// does not consult Obsidian config.
func (s *Service) GetDailyNotePath() string {
	today := time.Now()
	dateStr := today.Format("2006-01-02")
	return filepath.Join(s.opts.Folders.Daily, dateStr+".md")
}

// GetTodayDailyNotePath resolves today's daily note path by reading the
// Obsidian daily-notes plugin configuration.
func (s *Service) GetTodayDailyNotePath() (string, error) {
	settings, err := s.resolveDailyNoteSettings()
	if err != nil {
		return "", err
	}
	dateStr := formatObsidianDate(time.Now(), settings.format)
	return filepath.Join(settings.folder, dateStr+".md"), nil
}

// AppendToDaily appends a timestamped bullet to the "## Timeline" section of
// today's daily note, creating the note from a template or default structure
// when it does not exist yet.
func (s *Service) AppendToDaily(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	now := time.Now()
	settings, err := s.resolveDailyNoteSettings()
	if err != nil {
		s.log().Error("appendToDaily: resolve settings failed", "error", err)
		return false
	}

	dailyPath := filepath.Join(settings.folder, formatObsidianDate(now, settings.format)+".md")

	noteContent, ok := s.ReadFile(dailyPath)
	if !ok {
		noteContent = s.buildNewDailyNote(settings, now)
	}

	noteContent = ensureHeadingSection(noteContent, "## Timeline")
	noteContent = ensureHeadingSection(noteContent, "## Tracking")
	line := "- " + formatHourMinute(now) + " " + trimmed
	noteContent = appendLineToSection(noteContent, "## Timeline", line)

	return s.WriteFile(dailyPath, noteContent)
}

// UpsertDailyTracking inserts or updates a "- metric: value" line inside the
// "## Tracking" section of today's daily note.
func (s *Service) UpsertDailyTracking(metric, value string) bool {
	normalizedMetric := strings.TrimRight(strings.TrimSpace(metric), ":")
	normalizedValue := strings.TrimSpace(value)
	if normalizedMetric == "" || normalizedValue == "" {
		return false
	}

	now := time.Now()
	settings, err := s.resolveDailyNoteSettings()
	if err != nil {
		s.log().Error("upsertDailyTracking: resolve settings failed", "error", err)
		return false
	}

	dailyPath := filepath.Join(settings.folder, formatObsidianDate(now, settings.format)+".md")

	noteContent, ok := s.ReadFile(dailyPath)
	if !ok {
		noteContent = s.buildNewDailyNote(settings, now)
	}

	noteContent = ensureHeadingSection(noteContent, "## Tracking")
	noteContent = upsertTrackingLine(noteContent, "## Tracking", normalizedMetric, normalizedValue)

	return s.WriteFile(dailyPath, noteContent)
}

// MoveFile moves a file from fromPath to toPath (both vault-relative).
func (s *Service) MoveFile(fromPath, toPath string) bool {
	source, ok := s.resolveExistingPathWithinVault(fromPath, false)
	if !ok {
		return false
	}

	dest, ok := s.resolveWritablePathWithinVault(toPath)
	if !ok {
		return false
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		s.log().Error("vault move: mkdir failed", "error", err)
		return false
	}

	if !s.isWritablePathSafe(dest) {
		return false
	}

	if err := os.Rename(source, dest); err != nil {
		s.log().Error("vault move failed", "error", err)
		return false
	}
	return true
}

// GetStats returns aggregate statistics for all non-directory files in the vault.
func (s *Service) GetStats() (VaultStats, error) {
	files, err := s.listFilesRecursive("")
	if err != nil {
		return VaultStats{}, err
	}

	var stats VaultStats
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
	return stats, nil
}

// GetObsidianConfigSummary returns the cached (or freshly-read) Obsidian config
// summary. Pass forceRefresh=true to bypass the fingerprint cache.
func (s *Service) GetObsidianConfigSummary(forceRefresh bool) (ObsidianConfigSummary, error) {
	state, err := s.GetObsidianConfigState(forceRefresh)
	return state.Summary, err
}

// GetObsidianConfigState returns the Obsidian config summary plus cache metadata.
func (s *Service) GetObsidianConfigState(forceRefresh bool) (ObsidianConfigState, error) {
	fingerprint := s.computeVaultFingerprint()
	cacheHit := !forceRefresh && s.configCache != nil && s.configCache.fingerprint == fingerprint

	if cacheHit {
		return ObsidianConfigState{
			Summary:     s.configCache.summary,
			Fingerprint: fingerprint,
			CacheHit:    true,
		}, nil
	}

	summary, err := s.buildObsidianConfigSummary()
	if err != nil {
		return ObsidianConfigState{}, err
	}

	s.configCache = &obsidianConfigCache{fingerprint: fingerprint, summary: summary}

	return ObsidianConfigState{
		Summary:     summary,
		Fingerprint: fingerprint,
		CacheHit:    false,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal: recursive listing
// ---------------------------------------------------------------------------

func (s *Service) listFilesRecursive(folder string) ([]VaultFile, error) {
	var all []VaultFile
	items, err := s.ListFiles(folder)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		all = append(all, item)
		if item.IsDirectory {
			children, err := s.listFilesRecursive(item.Path)
			if err != nil {
				return nil, err
			}
			all = append(all, children...)
		}
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Internal: path security
// ---------------------------------------------------------------------------

// resolvePathWithinVault resolves a vault-relative input path to an absolute
// path that is guaranteed to be inside the vault root. Returns ("", false) for
// path-traversal attempts. When allowRoot is true an empty input is accepted
// and resolves to the vault root.
func (s *Service) resolvePathWithinVault(inputPath string, allowRoot bool) (string, bool) {
	trimmed := strings.TrimSpace(inputPath)
	if !allowRoot && trimmed == "" {
		return "", false
	}

	resolved := filepath.Join(s.vaultRoot, trimmed)
	rel, err := filepath.Rel(s.vaultRoot, resolved)
	if err != nil {
		return "", false
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return resolved, true
}

func (s *Service) resolveExistingPathWithinVault(inputPath string, allowRoot bool) (string, bool) {
	resolved, ok := s.resolvePathWithinVault(inputPath, allowRoot)
	if !ok {
		return "", false
	}
	if !s.isExistingPathSafe(resolved) {
		return "", false
	}
	return resolved, true
}

func (s *Service) resolveWritablePathWithinVault(inputPath string) (string, bool) {
	resolved, ok := s.resolvePathWithinVault(inputPath, false)
	if !ok {
		return "", false
	}
	if !s.isWritablePathSafe(resolved) {
		return "", false
	}
	return resolved, true
}

// isExistingPathSafe returns true when:
//  1. No segment of the path (relative to vault root) is a symlink.
//  2. Both the vault root and the target resolve (via os.EvalSymlinks) to paths
//     where the target is within the root.
func (s *Service) isExistingPathSafe(targetPath string) bool {
	if !s.hasNoSymlinkSegments(targetPath) {
		return false
	}

	rootReal, err := filepath.EvalSymlinks(s.vaultRoot)
	if err != nil {
		return false
	}
	targetReal, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return false
	}
	return isWithinRoot(rootReal, targetReal)
}

// isWritablePathSafe returns true when the path (which need not exist yet) is
// safe to write to: no symlink segments, nearest existing ancestor is within
// vault root, and if the target already exists it is not a symlink and resolves
// inside the vault root.
func (s *Service) isWritablePathSafe(targetPath string) bool {
	if !s.hasNoSymlinkSegments(targetPath) {
		return false
	}

	ancestor, ok := s.findNearestExistingAncestor(targetPath)
	if !ok {
		return false
	}

	rootReal, err := filepath.EvalSymlinks(s.vaultRoot)
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

	// If the target already exists, check it is not itself a symlink and
	// resolves inside the vault root.
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

// hasNoSymlinkSegments walks every path segment from the vault root to
// targetPath and returns false if any existing segment is a symlink.
func (s *Service) hasNoSymlinkSegments(targetPath string) bool {
	rel, err := filepath.Rel(s.vaultRoot, targetPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}

	segments := strings.Split(rel, string(filepath.Separator))
	current := s.vaultRoot

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

// findNearestExistingAncestor walks upward from targetPath until it finds a
// path component that exists on disk. Returns ("", false) when no ancestor
// exists at or below the filesystem root.
func (s *Service) findNearestExistingAncestor(targetPath string) (string, bool) {
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

// isWithinRoot returns true when candidatePath is the rootPath itself or a
// descendant of it.
func isWithinRoot(rootPath, candidatePath string) bool {
	rel, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ---------------------------------------------------------------------------
// Internal: Obsidian config
// ---------------------------------------------------------------------------

func (s *Service) buildObsidianConfigSummary() (ObsidianConfigSummary, error) {
	app := s.readObsidianJSON("app.json")
	daily := s.readObsidianJSON("daily-notes.json")
	templates := s.readObsidianJSON("templates.json")
	corePlugins := s.readObsidianJSONRaw("core-plugins.json")

	dailyFolder := asTrimmedString(daily["folder"])
	if dailyFolder == "" {
		dailyFolder = s.opts.Folders.Daily
	}

	dailyFormat := asTrimmedString(daily["format"])
	if dailyFormat == "" {
		dailyFormat = "YYYY-MM-DD"
	}

	dailyTemplate := asTrimmedString(daily["template"])
	pluginEnabled := jsonArrayContains(corePlugins, "daily-notes")

	newFileLocationRaw := asTrimmedString(app["newFileLocation"])
	newFileLocation := normalizeNewFileLocation(newFileLocationRaw)
	newFileFolder := asTrimmedString(app["newFileFolderPath"])

	attachmentFolderRaw := asTrimmedString(app["attachmentFolderPath"])
	useMarkdownLinks, _ := app["useMarkdownLinks"].(bool)
	templatesFolder := asTrimmedString(templates["folder"])

	var warnings []string
	if !pluginEnabled {
		warnings = append(warnings, "daily-notes core plugin is not enabled in core-plugins.json")
	}
	if newFileLocation == "folder" && newFileFolder == "" {
		warnings = append(warnings, "newFileLocation is set to folder but newFileFolderPath is empty")
	}
	if attachmentFolderRaw == "/" {
		warnings = append(warnings, "attachments are configured to save at the vault root")
	}

	obsidianConfigFound := app != nil || daily != nil || templates != nil || corePlugins != nil

	templateFileDisplay := dailyTemplate
	if templateFileDisplay == "" {
		templateFileDisplay = "(none)"
	}
	newFileFolderDisplay := newFileFolder
	if newFileFolderDisplay == "" {
		newFileFolderDisplay = "(not set)"
	}
	attachmentDisplay := attachmentFolderRaw
	if attachmentDisplay == "" {
		attachmentDisplay = "(current note folder)"
	}
	templatesFolderDisplay := templatesFolder
	if templatesFolderDisplay == "" {
		templatesFolderDisplay = "(not set)"
	}

	linkStyle := "wikilink"
	if useMarkdownLinks {
		linkStyle = "markdown"
	}

	return ObsidianConfigSummary{
		ObsidianConfigFound: obsidianConfigFound,
		DailyNotes: ObsidianDailyNotes{
			Folder:        dailyFolder,
			Format:        dailyFormat,
			TemplateFile:  templateFileDisplay,
			PluginEnabled: pluginEnabled,
		},
		NewNotes: ObsidianNewNotes{
			Location: newFileLocation,
			Folder:   newFileFolderDisplay,
		},
		Attachments: ObsidianAttachments{
			Folder: attachmentDisplay,
		},
		Links: ObsidianLinks{
			Style: linkStyle,
		},
		Templates: ObsidianTemplates{
			Folder: templatesFolderDisplay,
		},
		Warnings: warnings,
	}, nil
}

// readObsidianJSON reads a .obsidian/<fileName> JSON file and returns its
// top-level keys as a map. Returns nil on any error.
func (s *Service) readObsidianJSON(fileName string) map[string]interface{} {
	configPath, ok := s.resolveExistingPathWithinVault(filepath.Join(".obsidian", fileName), false)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

// readObsidianJSONRaw reads a .obsidian/<fileName> JSON file as a raw
// interface{} value (used for arrays such as core-plugins.json).
func (s *Service) readObsidianJSONRaw(fileName string) interface{} {
	configPath, ok := s.resolveExistingPathWithinVault(filepath.Join(".obsidian", fileName), false)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

// computeVaultFingerprint builds a cheap fingerprint of the vault's structural
// state used to invalidate the Obsidian config cache.
func (s *Service) computeVaultFingerprint() string {
	// Top-level non-hidden directories.
	var dirs []string
	entries, err := os.ReadDir(s.vaultRoot)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, e.Name())
			}
		}
		sort.Strings(dirs)
	}

	obsidianFiles := []string{"app.json", "daily-notes.json", "templates.json", "core-plugins.json"}
	sigs := make([]string, 0, len(obsidianFiles))
	for _, f := range obsidianFiles {
		relPath := filepath.Join(".obsidian", f)
		sigs = append(sigs, s.getFileSignature(relPath))
	}

	return fmt.Sprintf("dirs=%s;obsidian=%s",
		strings.Join(dirs, ","),
		strings.Join(sigs, "|"),
	)
}

// getFileSignature returns a "<path>:<size>:<mtime_ms>" string for a
// vault-relative file, or "<path>:missing" when the file does not exist.
func (s *Service) getFileSignature(relativePath string) string {
	absPath, ok := s.resolveExistingPathWithinVault(relativePath, false)
	if !ok {
		return relativePath + ":missing"
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return relativePath + ":missing"
	}
	return fmt.Sprintf("%s:%d:%d", relativePath, info.Size(), info.ModTime().UnixMilli())
}

// resolveDailyNoteSettings reads the Obsidian daily-notes plugin config and
// merges it with configured defaults.
func (s *Service) resolveDailyNoteSettings() (dailyNoteSettings, error) {
	daily := s.readObsidianJSON("daily-notes.json")

	folder := asTrimmedString(daily["folder"])
	if folder == "" {
		folder = s.opts.Folders.Daily
	}

	format := asTrimmedString(daily["format"])
	if format == "" {
		format = s.opts.DailyNoteFormat
	}

	return dailyNoteSettings{
		folder:       folder,
		format:       format,
		templateFile: asTrimmedString(daily["template"]),
	}, nil
}

// buildNewDailyNote produces the initial content for a new daily note, using a
// template file when configured.
func (s *Service) buildNewDailyNote(settings dailyNoteSettings, now time.Time) string {
	title := formatObsidianDate(now, settings.format)

	if settings.templateFile != "" {
		templateContent, ok := s.ReadFile(settings.templateFile)
		if ok && strings.TrimSpace(templateContent) != "" {
			content := templateContent
			content = ensureHeadingSection(content, "## Timeline")
			content = ensureHeadingSection(content, "## Tracking")
			return content
		}
	}

	return strings.Join([]string{
		"# " + title,
		"",
		"## Timeline",
		"",
		"## Tracking",
		"- Mood:",
		"- Energy:",
		"- Focus:",
		"- Sleep:",
		"",
	}, "\n")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func normalizeSearchMode(mode VaultSearchMode) VaultSearchMode {
	switch mode {
	case SearchModeContent, SearchModeBoth, SearchModeName:
		return mode
	default:
		return SearchModeName
	}
}

func normalizeTag(tag string) string {
	trimmed := strings.ToLower(strings.TrimSpace(tag))
	if trimmed == "" {
		return "#"
	}
	if strings.HasPrefix(trimmed, "#") {
		return trimmed
	}
	return "#" + trimmed
}

func asTrimmedString(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func normalizeNewFileLocation(v string) string {
	switch v {
	case "current", "folder", "root":
		return v
	case "":
		return "current"
	default:
		return "unknown"
	}
}

// jsonArrayContains returns true when raw is a JSON array ([]interface{}) that
// contains the string value.
func jsonArrayContains(raw interface{}, value string) bool {
	arr, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range arr {
		if s, ok := item.(string); ok && s == value {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Markdown helpers
// ---------------------------------------------------------------------------

// normalizeLineEndings converts CRLF to LF.
func normalizeLineEndings(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// ensureHeadingSection returns content with heading appended (preceded by a
// blank line) when it is not already present.
func ensureHeadingSection(content, heading string) string {
	normalized := normalizeLineEndings(content)
	lines := strings.Split(normalized, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == heading {
			return normalized
		}
	}

	trimmed := strings.TrimRight(normalized, "\n\r\t ")
	if trimmed == "" {
		return heading + "\n"
	}
	return trimmed + "\n\n" + heading + "\n"
}

// sectionBounds returns the (start, end) line indices for the section opened
// by heading. end is the index of the next ## heading or len(lines).
// Returns (-1, -1) when the heading is not found.
func sectionBounds(lines []string, heading string) (start, end int) {
	start = -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i
			break
		}
	}
	if start == -1 {
		return -1, -1
	}

	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return start, end
}

// appendLineToSection inserts lineToAdd just before the end of the section
// opened by heading.
func appendLineToSection(content, heading, lineToAdd string) string {
	normalized := normalizeLineEndings(ensureHeadingSection(content, heading))
	lines := strings.Split(normalized, "\n")
	start, end := sectionBounds(lines, heading)
	if start == -1 {
		return normalized
	}

	// Splice the new line in at position `end`.
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:end]...)
	result = append(result, lineToAdd)
	result = append(result, lines[end:]...)
	return strings.Join(result, "\n")
}

// upsertTrackingLine inserts or updates a "- metric: value" line within the
// section opened by heading.
func upsertTrackingLine(content, heading, metric, value string) string {
	normalized := normalizeLineEndings(ensureHeadingSection(content, heading))
	lines := strings.Split(normalized, "\n")
	start, end := sectionBounds(lines, heading)
	if start == -1 {
		return normalized
	}

	desired := "- " + metric + ": " + value
	targetLower := strings.ToLower(metric)

	// Try to find an existing line with the same metric key.
	// Match pattern: optional whitespace, "- ", key, ":"
	for i := start + 1; i < end; i++ {
		// Extract the key from lines like "- Mood: ..." or "  - mood: ..."
		trimLine := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimLine, "- ") {
			continue
		}
		rest := trimLine[2:] // strip leading "- "
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(rest[:colonIdx])
		if strings.ToLower(key) == targetLower {
			lines[i] = desired
			return strings.Join(lines, "\n")
		}
	}

	// Key not found – insert before end of section.
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:end]...)
	result = append(result, desired)
	result = append(result, lines[end:]...)
	return strings.Join(result, "\n")
}

// formatObsidianDate formats t using an Obsidian moment.js-style pattern.
// Supported tokens: YYYY YY MMMM MMM MM M DD D dddd ddd HH H mm m
func formatObsidianDate(t time.Time, pattern string) string {
	monthNamesFull := [12]string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}
	monthNamesShort := [12]string{
		"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}
	dayNamesFull := [7]string{
		"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
	}
	dayNamesShort := [7]string{
		"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat",
	}

	month := int(t.Month()) - 1 // 0-indexed
	dayOfWeek := int(t.Weekday()) // 0=Sunday
	year := t.Year()

	tokenMap := map[string]string{
		"YYYY": fmt.Sprintf("%04d", year),
		"YY":   fmt.Sprintf("%04d", year)[2:],
		"MMMM": monthNamesFull[month],
		"MMM":  monthNamesShort[month],
		"MM":   fmt.Sprintf("%02d", month+1),
		"M":    fmt.Sprintf("%d", month+1),
		"DD":   fmt.Sprintf("%02d", t.Day()),
		"D":    fmt.Sprintf("%d", t.Day()),
		"dddd": dayNamesFull[dayOfWeek],
		"ddd":  dayNamesShort[dayOfWeek],
		"HH":   fmt.Sprintf("%02d", t.Hour()),
		"H":    fmt.Sprintf("%d", t.Hour()),
		"mm":   fmt.Sprintf("%02d", t.Minute()),
		"m":    fmt.Sprintf("%d", t.Minute()),
	}

	// Process tokens longest-first to avoid partial matches (e.g. MMMM before MMM).
	orderedTokens := []string{
		"YYYY", "MMMM", "dddd", "MMM", "ddd", "MM", "DD", "HH", "mm", "YY", "M", "D", "H", "m",
	}

	var result strings.Builder
	i := 0
	for i < len(pattern) {
		matched := false
		for _, tok := range orderedTokens {
			if strings.HasPrefix(pattern[i:], tok) {
				result.WriteString(tokenMap[tok])
				i += len(tok)
				matched = true
				break
			}
		}
		if !matched {
			result.WriteByte(pattern[i])
			i++
		}
	}
	return result.String()
}

// formatHourMinute returns "HH:mm" for the given time.
func formatHourMinute(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}
