package vault

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/config"
)

// newTestService creates a vault Service backed by t.TempDir().
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	svc := New(Options{
		VaultPath:       root,
		DailyNoteFormat: "YYYY-MM-DD",
		Folders: config.VaultFolders{
			Inbox:     "inbox",
			Daily:     "daily",
			Journal:   "daily",
			Projects:  "projects",
			Areas:     "areas",
			Resources: "resources",
			Archive:   "archive",
		},
		Logger: slog.Default(),
	})
	return svc, root
}

// seedFile writes a file relative to root, creating parent dirs as needed.
func seedFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// 1. Initialize
// ---------------------------------------------------------------------------

func TestInitialize_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-vault")
	svc := New(Options{
		VaultPath:       dir,
		DailyNoteFormat: "YYYY-MM-DD",
		Folders:         config.VaultFolders{Daily: "daily"},
		Logger:          slog.Default(),
	})

	if err := svc.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("vault directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("vault path is not a directory")
	}
}

func TestInitialize_Idempotent(t *testing.T) {
	svc, root := newTestService(t)
	if err := svc.Initialize(); err != nil {
		t.Fatalf("first Initialize failed: %v", err)
	}
	if err := svc.Initialize(); err != nil {
		t.Fatalf("second Initialize failed: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatal("vault directory does not exist after double Initialize")
	}
}

// ---------------------------------------------------------------------------
// 2. ReadFile
// ---------------------------------------------------------------------------

func TestReadFile_ExistingFile(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "notes/hello.md", "hello world")

	content, ok := svc.ReadFile("notes/hello.md")
	if !ok {
		t.Fatal("ReadFile returned false for existing file")
	}
	if content != "hello world" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestReadFile_NonExistent(t *testing.T) {
	svc, _ := newTestService(t)

	_, ok := svc.ReadFile("no-such-file.md")
	if ok {
		t.Fatal("ReadFile returned true for non-existent file")
	}
}

func TestReadFile_BlocksPathTraversal(t *testing.T) {
	svc, _ := newTestService(t)

	_, ok := svc.ReadFile("../../etc/passwd")
	if ok {
		t.Fatal("ReadFile should block path traversal")
	}
}

func TestReadFile_EmptyPathReturnsFalse(t *testing.T) {
	svc, _ := newTestService(t)

	_, ok := svc.ReadFile("")
	if ok {
		t.Fatal("ReadFile should return false for empty path")
	}
}

// ---------------------------------------------------------------------------
// 3. WriteFile
// ---------------------------------------------------------------------------

func TestWriteFile_CreatesFile(t *testing.T) {
	svc, root := newTestService(t)

	ok := svc.WriteFile("hello.md", "hello")
	if !ok {
		t.Fatal("WriteFile returned false")
	}

	data, err := os.ReadFile(filepath.Join(root, "hello.md"))
	if err != nil {
		t.Fatalf("could not read written file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	svc, root := newTestService(t)

	ok := svc.WriteFile("a/b/c/deep.md", "deep content")
	if !ok {
		t.Fatal("WriteFile returned false")
	}

	data, err := os.ReadFile(filepath.Join(root, "a/b/c/deep.md"))
	if err != nil {
		t.Fatalf("could not read written file: %v", err)
	}
	if string(data) != "deep content" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestWriteFile_BlocksPathTraversal(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.WriteFile("../escape.md", "bad")
	if ok {
		t.Fatal("WriteFile should block path traversal")
	}
}

func TestWriteFile_BlocksSymlinkTarget(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()

	// Create a symlink inside the vault pointing outside.
	link := filepath.Join(root, "evil-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	ok := svc.WriteFile("evil-link/escape.md", "bad")
	if ok {
		t.Fatal("WriteFile should block writes through symlinks")
	}
}

func TestWriteFile_OverwritesExisting(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "overwrite.md", "old")

	ok := svc.WriteFile("overwrite.md", "new")
	if !ok {
		t.Fatal("WriteFile returned false")
	}

	data, err := os.ReadFile(filepath.Join(root, "overwrite.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("expected %q, got %q", "new", string(data))
	}
}

// ---------------------------------------------------------------------------
// 4. AppendToFile
// ---------------------------------------------------------------------------

func TestAppendToFile_AppendsToExisting(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "append.md", "line1\n")

	ok := svc.AppendToFile("append.md", "line2\n")
	if !ok {
		t.Fatal("AppendToFile returned false")
	}

	content, ok := svc.ReadFile("append.md")
	if !ok {
		t.Fatal("ReadFile after append returned false")
	}
	if content != "line1\nline2\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestAppendToFile_CreatesIfNotExists(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.AppendToFile("new-file.md", "first line\n")
	if !ok {
		t.Fatal("AppendToFile returned false for new file")
	}

	content, ok := svc.ReadFile("new-file.md")
	if !ok {
		t.Fatal("ReadFile returned false after create-via-append")
	}
	if content != "first line\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}

// ---------------------------------------------------------------------------
// 5. ListFiles
// ---------------------------------------------------------------------------

func TestListFiles_ListsDirectoryContents(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "alpha.md", "a")
	seedFile(t, root, "beta.md", "b")

	files, err := svc.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestListFiles_SkipsHiddenFiles(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, ".hidden", "h")
	seedFile(t, root, "visible.md", "v")

	files, err := svc.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (hidden skipped), got %d", len(files))
	}
	if files[0].Name != "visible.md" {
		t.Fatalf("expected visible.md, got %s", files[0].Name)
	}
}

func TestListFiles_SortsDirsFirst(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "zebra.md", "z")
	seedFile(t, root, "alpha-dir/file.md", "a")

	files, err := svc.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(files))
	}
	if !files[0].IsDirectory {
		t.Fatalf("expected first entry to be a directory, got %q", files[0].Name)
	}
	if files[0].Name != "alpha-dir" {
		t.Fatalf("expected alpha-dir first, got %s", files[0].Name)
	}
}

func TestListFiles_EmptyDir(t *testing.T) {
	svc, _ := newTestService(t)

	files, err := svc.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files in empty vault, got %d", len(files))
	}
}

func TestListFiles_Subfolder(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "sub/one.md", "1")
	seedFile(t, root, "sub/two.md", "2")
	seedFile(t, root, "root.md", "r")

	files, err := svc.ListFiles("sub")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files in sub, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// 6. SearchFiles
// ---------------------------------------------------------------------------

func TestSearchFiles_NameSearch(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "meeting-notes.md", "some content")
	seedFile(t, root, "todo.md", "other content")

	results, err := svc.SearchFiles("meeting", "", SearchModeName)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "meeting-notes.md" {
		t.Fatalf("expected meeting-notes.md, got %s", results[0].Name)
	}
}

func TestSearchFiles_ContentSearch(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "a.md", "the quick brown fox")
	seedFile(t, root, "b.md", "lazy dog")

	results, err := svc.SearchFiles("brown fox", "", SearchModeContent)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "a.md" {
		t.Fatalf("expected a.md, got %s", results[0].Name)
	}
}

func TestSearchFiles_BothMode(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "fox-story.md", "no mention here")
	seedFile(t, root, "other.md", "the quick brown fox")

	results, err := svc.SearchFiles("fox", "", SearchModeBoth)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (name match + content match), got %d", len(results))
	}
}

func TestSearchFiles_CaseInsensitive(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "README.md", "Hello World")

	results, err := svc.SearchFiles("readme", "", SearchModeName)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive name search, got %d", len(results))
	}
}

func TestSearchFiles_NoResults(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "a.md", "hello")

	results, err := svc.SearchFiles("zzzzz", "", SearchModeName)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// 7. FindBacklinks
// ---------------------------------------------------------------------------

func TestFindBacklinks_FindsWikiLinks(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "note-a.md", "See [[Target Note]] for info")
	seedFile(t, root, "note-b.md", "Also links to [[target note]]")
	seedFile(t, root, "note-c.md", "No links here")

	results, err := svc.FindBacklinks("Target Note", "")
	if err != nil {
		t.Fatalf("FindBacklinks error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 backlinks, got %d", len(results))
	}
}

func TestFindBacklinks_NoMatches(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "note.md", "[[Other Note]]")

	results, err := svc.FindBacklinks("Unlinked", "")
	if err != nil {
		t.Fatalf("FindBacklinks error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 backlinks, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// 8. SearchByTag
// ---------------------------------------------------------------------------

func TestSearchByTag_FindsTaggedFiles(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "tagged.md", "Some content #project and more")
	seedFile(t, root, "untagged.md", "No tags here")

	results, err := svc.SearchByTag("project", "")
	if err != nil {
		t.Fatalf("SearchByTag error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "tagged.md" {
		t.Fatalf("expected tagged.md, got %s", results[0].Name)
	}
}

func TestSearchByTag_HashPrefix(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "note.md", "This is #important stuff")

	// Pass tag with leading #.
	results, err := svc.SearchByTag("#important", "")
	if err != nil {
		t.Fatalf("SearchByTag error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with # prefix, got %d", len(results))
	}
}

func TestSearchByTag_NoMatches(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "note.md", "content #alpha")

	results, err := svc.SearchByTag("beta", "")
	if err != nil {
		t.Fatalf("SearchByTag error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// 9. MoveFile
// ---------------------------------------------------------------------------

func TestMoveFile_MovesFile(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "src.md", "content")

	ok := svc.MoveFile("src.md", "dest.md")
	if !ok {
		t.Fatal("MoveFile returned false")
	}

	// Source should not exist.
	if _, err := os.Stat(filepath.Join(root, "src.md")); !os.IsNotExist(err) {
		t.Fatal("source file still exists after move")
	}

	// Destination should exist.
	data, err := os.ReadFile(filepath.Join(root, "dest.md"))
	if err != nil {
		t.Fatalf("dest file not found: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestMoveFile_CreatesDestDirs(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "flat.md", "data")

	ok := svc.MoveFile("flat.md", "deep/nested/file.md")
	if !ok {
		t.Fatal("MoveFile returned false")
	}

	data, err := os.ReadFile(filepath.Join(root, "deep/nested/file.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestMoveFile_BlocksTraversal(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "file.md", "data")

	ok := svc.MoveFile("file.md", "../../escaped.md")
	if ok {
		t.Fatal("MoveFile should block path traversal on destination")
	}

	ok = svc.MoveFile("../../etc/passwd", "stolen.md")
	if ok {
		t.Fatal("MoveFile should block path traversal on source")
	}
}

func TestMoveFile_SourceNotExist(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.MoveFile("nonexistent.md", "dest.md")
	if ok {
		t.Fatal("MoveFile should return false for non-existent source")
	}
}

// ---------------------------------------------------------------------------
// 10. GetDailyNotePath
// ---------------------------------------------------------------------------

func TestGetDailyNotePath_ReturnsCorrectPath(t *testing.T) {
	svc, _ := newTestService(t)

	path := svc.GetDailyNotePath()
	today := time.Now().Format("2006-01-02")
	expected := filepath.Join("daily", today+".md")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

// ---------------------------------------------------------------------------
// 11. GetTodayDailyNotePath
// ---------------------------------------------------------------------------

func TestGetTodayDailyNotePath_ObsidianFormat(t *testing.T) {
	svc, _ := newTestService(t)

	path, err := svc.GetTodayDailyNotePath()
	if err != nil {
		t.Fatalf("GetTodayDailyNotePath error: %v", err)
	}

	// With no .obsidian config, it falls back to the configured defaults.
	today := time.Now().Format("2006-01-02")
	expected := filepath.Join("daily", today+".md")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestGetTodayDailyNotePath_WithObsidianConfig(t *testing.T) {
	svc, root := newTestService(t)

	// Write a daily-notes.json Obsidian config.
	seedFile(t, root, ".obsidian/daily-notes.json", `{"folder": "journal", "format": "YYYY-MM-DD"}`)

	path, err := svc.GetTodayDailyNotePath()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	expected := filepath.Join("journal", today+".md")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

// ---------------------------------------------------------------------------
// 12. AppendToDaily
// ---------------------------------------------------------------------------

func TestAppendToDaily_CreatesNewDailyNote(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.AppendToDaily("first entry")
	if !ok {
		t.Fatal("AppendToDaily returned false")
	}

	// Read back the daily note.
	today := time.Now().Format("2006-01-02")
	dailyPath := filepath.Join("daily", today+".md")
	content, found := svc.ReadFile(dailyPath)
	if !found {
		t.Fatal("daily note file was not created")
	}

	// Should have default template structure.
	if !strings.Contains(content, "## Timeline") {
		t.Fatal("daily note missing ## Timeline section")
	}
	if !strings.Contains(content, "## Tracking") {
		t.Fatal("daily note missing ## Tracking section")
	}
	if !strings.Contains(content, "first entry") {
		t.Fatal("daily note missing appended entry")
	}
}

func TestAppendToDaily_AppendsTimestampedEntry(t *testing.T) {
	svc, _ := newTestService(t)

	svc.AppendToDaily("entry one")
	svc.AppendToDaily("entry two")

	today := time.Now().Format("2006-01-02")
	dailyPath := filepath.Join("daily", today+".md")
	content, found := svc.ReadFile(dailyPath)
	if !found {
		t.Fatal("daily note not found")
	}

	if !strings.Contains(content, "entry one") {
		t.Fatal("missing first entry")
	}
	if !strings.Contains(content, "entry two") {
		t.Fatal("missing second entry")
	}

	// Entries should be prefixed with "- HH:mm ".
	hour := time.Now().Format("15:")
	if !strings.Contains(content, "- "+hour) {
		t.Fatalf("entries should be timestamped; content:\n%s", content)
	}
}

func TestAppendToDaily_EmptyContentReturnsFalse(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.AppendToDaily("")
	if ok {
		t.Fatal("AppendToDaily should return false for empty content")
	}

	ok = svc.AppendToDaily("   ")
	if ok {
		t.Fatal("AppendToDaily should return false for whitespace-only content")
	}
}

func TestAppendToDaily_WithTemplate(t *testing.T) {
	svc, root := newTestService(t)

	// Set up Obsidian config with a template.
	templateContent := "# {{date}}\n\nCustom template body\n"
	seedFile(t, root, "templates/daily.md", templateContent)
	seedFile(t, root, ".obsidian/daily-notes.json", `{"folder": "daily", "format": "YYYY-MM-DD", "template": "templates/daily.md"}`)

	ok := svc.AppendToDaily("test entry")
	if !ok {
		t.Fatal("AppendToDaily returned false with template")
	}

	today := time.Now().Format("2006-01-02")
	content, found := svc.ReadFile(filepath.Join("daily", today+".md"))
	if !found {
		t.Fatal("daily note not created")
	}

	if !strings.Contains(content, "Custom template body") {
		t.Fatalf("template content not used; got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// 13. UpsertDailyTracking
// ---------------------------------------------------------------------------

func TestUpsertDailyTracking_InsertsNewMetric(t *testing.T) {
	svc, _ := newTestService(t)

	// Create the daily note first.
	svc.AppendToDaily("init")

	ok := svc.UpsertDailyTracking("Mood", "8")
	if !ok {
		t.Fatal("UpsertDailyTracking returned false")
	}

	today := time.Now().Format("2006-01-02")
	content, found := svc.ReadFile(filepath.Join("daily", today+".md"))
	if !found {
		t.Fatal("daily note not found")
	}

	if !strings.Contains(content, "- Mood: 8") {
		t.Fatalf("expected '- Mood: 8' in content:\n%s", content)
	}
}

func TestUpsertDailyTracking_UpdatesExistingMetric(t *testing.T) {
	svc, _ := newTestService(t)

	svc.AppendToDaily("init")
	svc.UpsertDailyTracking("Mood", "5")
	svc.UpsertDailyTracking("Mood", "9")

	today := time.Now().Format("2006-01-02")
	content, found := svc.ReadFile(filepath.Join("daily", today+".md"))
	if !found {
		t.Fatal("daily note not found")
	}

	// Should contain the updated value, not both.
	if !strings.Contains(content, "- Mood: 9") {
		t.Fatalf("expected '- Mood: 9', got:\n%s", content)
	}
	if strings.Count(content, "- Mood:") != 1 {
		t.Fatalf("expected exactly one Mood line, got:\n%s", content)
	}
}

func TestUpsertDailyTracking_EmptyMetricReturnsFalse(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.UpsertDailyTracking("", "value")
	if ok {
		t.Fatal("UpsertDailyTracking should return false for empty metric")
	}
}

func TestUpsertDailyTracking_EmptyValueReturnsFalse(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.UpsertDailyTracking("Mood", "")
	if ok {
		t.Fatal("UpsertDailyTracking should return false for empty value")
	}
}

func TestUpsertDailyTracking_CreatesNoteIfNotExist(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.UpsertDailyTracking("Sleep", "7h")
	if !ok {
		t.Fatal("UpsertDailyTracking returned false when creating new note")
	}

	today := time.Now().Format("2006-01-02")
	content, found := svc.ReadFile(filepath.Join("daily", today+".md"))
	if !found {
		t.Fatal("daily note not found")
	}
	if !strings.Contains(content, "- Sleep: 7h") {
		t.Fatalf("expected '- Sleep: 7h', got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// 14. GetStats
// ---------------------------------------------------------------------------

func TestGetStats_CountsFilesAndSizes(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "a.md", "hello")      // 5 bytes
	seedFile(t, root, "b.md", "world")      // 5 bytes
	seedFile(t, root, "sub/c.md", "foobar") // 6 bytes

	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}

	// TotalFiles includes directories in the recursive listing.
	// The service lists: a.md, b.md, sub (dir), sub/c.md = 4 entries.
	if stats.TotalFiles < 3 {
		t.Fatalf("expected at least 3 total files, got %d", stats.TotalFiles)
	}

	// TotalSize only counts non-directory files: 5 + 5 + 6 = 16
	if stats.TotalSize != 16 {
		t.Fatalf("expected total size 16, got %d", stats.TotalSize)
	}

	if stats.LastModified == nil {
		t.Fatal("LastModified should not be nil")
	}
}

func TestGetStats_EmptyVault(t *testing.T) {
	svc, _ := newTestService(t)

	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}

	if stats.TotalFiles != 0 {
		t.Fatalf("expected 0 files, got %d", stats.TotalFiles)
	}
	if stats.TotalSize != 0 {
		t.Fatalf("expected 0 size, got %d", stats.TotalSize)
	}
	if stats.LastModified != nil {
		t.Fatal("LastModified should be nil for empty vault")
	}
}

// ---------------------------------------------------------------------------
// 15. Path security: symlink detection, traversal prevention
// ---------------------------------------------------------------------------

func TestPathSecurity_SymlinkDetection(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()

	// Create a file outside the vault.
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the vault pointing to the outside file.
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// ReadFile should refuse to follow the symlink.
	_, ok := svc.ReadFile("link.txt")
	if ok {
		t.Fatal("ReadFile should not follow symlinks")
	}

	// WriteFile should refuse to write through the symlink.
	ok = svc.WriteFile("link.txt", "overwrite")
	if ok {
		t.Fatal("WriteFile should not write through symlinks")
	}
}

func TestPathSecurity_SymlinkDirectory(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()

	// Create a symlink directory inside the vault.
	link := filepath.Join(root, "linked-dir")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// ListFiles should skip symlink entries.
	files, err := svc.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	for _, f := range files {
		if f.Name == "linked-dir" {
			t.Fatal("ListFiles should skip symlink entries")
		}
	}
}

func TestPathSecurity_TraversalVariants(t *testing.T) {
	svc, _ := newTestService(t)

	traversalPaths := []string{
		"../escape",
		"../../etc/passwd",
		"foo/../../escape",
		"foo/../../../escape",
	}

	for _, p := range traversalPaths {
		_, ok := svc.ReadFile(p)
		if ok {
			t.Fatalf("ReadFile should block traversal path: %s", p)
		}

		ok = svc.WriteFile(p, "bad")
		if ok {
			t.Fatalf("WriteFile should block traversal path: %s", p)
		}
	}
}

func TestPathSecurity_IntermediateSymlink(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()

	// Create legitimate directory, then replace it with symlink.
	dir := filepath.Join(root, "legit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedFile(t, root, "legit/file.md", "data")

	// Remove the directory and replace with a symlink.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	// ReadFile through the intermediate symlink should fail.
	_, ok := svc.ReadFile("legit/file.md")
	if ok {
		t.Fatal("ReadFile should detect intermediate symlink segments")
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestWriteFile_EmptyPathReturnsFalse(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.WriteFile("", "content")
	if ok {
		t.Fatal("WriteFile should return false for empty path")
	}
}

func TestMoveFile_EmptyPathsReturnFalse(t *testing.T) {
	svc, _ := newTestService(t)

	ok := svc.MoveFile("", "dest.md")
	if ok {
		t.Fatal("MoveFile should return false for empty source")
	}

	ok = svc.MoveFile("src.md", "")
	if ok {
		t.Fatal("MoveFile should return false for empty dest")
	}
}

func TestSearchFiles_InSubfolder(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "projects/a.md", "alpha content")
	seedFile(t, root, "projects/b.md", "beta content")
	seedFile(t, root, "other/c.md", "alpha content")

	results, err := svc.SearchFiles("alpha", "projects", SearchModeContent)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result in subfolder, got %d", len(results))
	}
}

func TestListFiles_NonExistentFolder(t *testing.T) {
	svc, _ := newTestService(t)

	files, err := svc.ListFiles("nonexistent")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if files != nil {
		t.Fatalf("expected nil for non-existent folder, got %v", files)
	}
}

func TestFormatObsidianDate(t *testing.T) {
	// Test the date formatting function directly.
	// Use a fixed time: 2025-03-15 14:05 (Saturday)
	fixedTime := time.Date(2025, 3, 15, 14, 5, 0, 0, time.UTC)

	tests := []struct {
		pattern  string
		expected string
	}{
		{"YYYY-MM-DD", "2025-03-15"},
		{"YYYY/MM/DD", "2025/03/15"},
		{"DD-MM-YYYY", "15-03-2025"},
		{"MMMM DD, YYYY", "March 15, 2025"},
		{"dddd", "Saturday"},
		{"ddd", "Sat"},
		{"YYYY-MM-DD HH:mm", "2025-03-15 14:05"},
	}

	for _, tt := range tests {
		got := formatObsidianDate(fixedTime, tt.pattern)
		if got != tt.expected {
			t.Errorf("formatObsidianDate(%q) = %q, want %q", tt.pattern, got, tt.expected)
		}
	}
}

func TestNormalizeSearchMode(t *testing.T) {
	if normalizeSearchMode(SearchModeName) != SearchModeName {
		t.Fatal("name mode should pass through")
	}
	if normalizeSearchMode(SearchModeContent) != SearchModeContent {
		t.Fatal("content mode should pass through")
	}
	if normalizeSearchMode(SearchModeBoth) != SearchModeBoth {
		t.Fatal("both mode should pass through")
	}
	if normalizeSearchMode("invalid") != SearchModeName {
		t.Fatal("invalid mode should default to name")
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"project", "#project"},
		{"#project", "#project"},
		{"  Project  ", "#project"},
		{"#Project", "#project"},
		{"", "#"},
	}

	for _, tt := range tests {
		got := normalizeTag(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeTag(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStop_NoOp(t *testing.T) {
	svc, _ := newTestService(t)
	// Should not panic.
	svc.Stop()
}

func TestReadFile_SymlinkToFileOutsideVault(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(outsideFile, []byte("secret data"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "sneaky.md")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	_, ok := svc.ReadFile("sneaky.md")
	if ok {
		t.Fatal("ReadFile should block symlinks pointing outside vault")
	}
}

func TestWriteFile_SymlinkFileTarget(t *testing.T) {
	svc, root := newTestService(t)
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "target.md")
	if err := os.WriteFile(outsideFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "symfile.md")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	ok := svc.WriteFile("symfile.md", "overwritten")
	if ok {
		t.Fatal("WriteFile should refuse to overwrite through a symlink")
	}

	// Verify original file was not modified.
	data, _ := os.ReadFile(outsideFile)
	if string(data) != "original" {
		t.Fatal("symlink target file was unexpectedly modified")
	}
}

func TestUpsertDailyTracking_CaseInsensitiveKeyMatch(t *testing.T) {
	svc, _ := newTestService(t)

	// The default template creates "- Mood:" (capitalized).
	// Upserting "mood" (lowercase) should update the same line.
	svc.AppendToDaily("init")
	svc.UpsertDailyTracking("Mood", "5")
	svc.UpsertDailyTracking("mood", "10")

	today := time.Now().Format("2006-01-02")
	content, found := svc.ReadFile(filepath.Join("daily", today+".md"))
	if !found {
		t.Fatal("daily note not found")
	}

	// The default template has "- Mood:" and we upserted with lowercase "mood".
	// The upsert logic matches case-insensitively, so there should be exactly
	// one line matching mood (the updated one).
	count := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "- mood:") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 mood line, got %d in:\n%s", count, content)
	}
}

func TestSearchFiles_ContentSearchSkipsDirectories(t *testing.T) {
	svc, root := newTestService(t)
	seedFile(t, root, "dir/nested.md", "findme")

	results, err := svc.SearchFiles("findme", "", SearchModeContent)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	// Should find nested.md but not the "dir" directory.
	for _, r := range results {
		if r.IsDirectory {
			t.Fatal("content search should skip directories")
		}
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
