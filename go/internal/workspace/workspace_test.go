package workspace

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// newTestWorkspace creates a Workspace backed by t.TempDir().
func newTestWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	ws := New(root, slog.Default())
	return ws, root
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
	dir := filepath.Join(t.TempDir(), "new-workspace")
	ws := New(dir, slog.Default())

	if err := ws.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("workspace directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("workspace path is not a directory")
	}
}

func TestInitialize_Idempotent(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := ws.Initialize(); err != nil {
		t.Fatalf("first Initialize failed: %v", err)
	}
	if err := ws.Initialize(); err != nil {
		t.Fatalf("second Initialize failed: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatal("workspace directory does not exist after double Initialize")
	}
}

// ---------------------------------------------------------------------------
// 2. ReadFile
// ---------------------------------------------------------------------------

func TestReadFile_ExistingFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "notes/hello.md", "hello world")

	content, ok := ws.ReadFile("notes/hello.md")
	if !ok {
		t.Fatal("ReadFile returned false for existing file")
	}
	if content != "hello world" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestReadFile_NonExistent(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	_, ok := ws.ReadFile("no-such-file.md")
	if ok {
		t.Fatal("ReadFile returned true for non-existent file")
	}
}

func TestReadFile_BlocksPathTraversal(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	_, ok := ws.ReadFile("../../etc/passwd")
	if ok {
		t.Fatal("ReadFile should block path traversal")
	}
}

func TestReadFile_EmptyPath(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	_, ok := ws.ReadFile("")
	if ok {
		t.Fatal("ReadFile should return false for empty path")
	}
}

// ---------------------------------------------------------------------------
// 3. WriteFile
// ---------------------------------------------------------------------------

func TestWriteFile_CreatesFile(t *testing.T) {
	ws, root := newTestWorkspace(t)

	ok := ws.WriteFile("hello.md", "hello")
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
	ws, root := newTestWorkspace(t)

	ok := ws.WriteFile("a/b/c/deep.md", "deep content")
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
	ws, _ := newTestWorkspace(t)

	ok := ws.WriteFile("../escape.md", "bad")
	if ok {
		t.Fatal("WriteFile should block path traversal")
	}
}

func TestWriteFile_BlocksSymlinkTarget(t *testing.T) {
	ws, root := newTestWorkspace(t)
	outside := t.TempDir()

	link := filepath.Join(root, "evil-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	ok := ws.WriteFile("evil-link/escape.md", "bad")
	if ok {
		t.Fatal("WriteFile should block writes through symlinks")
	}
}

func TestWriteFile_OverwritesExisting(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "overwrite.md", "old")

	ok := ws.WriteFile("overwrite.md", "new")
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "append.md", "line1\n")

	ok := ws.AppendToFile("append.md", "line2\n")
	if !ok {
		t.Fatal("AppendToFile returned false")
	}

	content, ok := ws.ReadFile("append.md")
	if !ok {
		t.Fatal("ReadFile after append returned false")
	}
	if content != "line1\nline2\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestAppendToFile_CreatesIfNotExists(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	ok := ws.AppendToFile("new-file.md", "first line\n")
	if !ok {
		t.Fatal("AppendToFile returned false for new file")
	}

	content, ok := ws.ReadFile("new-file.md")
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

func TestListFiles_DirectoryContents(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "alpha.md", "a")
	seedFile(t, root, "beta.md", "b")

	files, err := ws.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestListFiles_SkipsHiddenFiles(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, ".hidden", "h")
	seedFile(t, root, "visible.md", "v")

	files, err := ws.ListFiles("")
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "zebra.md", "z")
	seedFile(t, root, "alpha-dir/file.md", "a")

	files, err := ws.ListFiles("")
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
	ws, _ := newTestWorkspace(t)

	files, err := ws.ListFiles("")
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files in empty workspace, got %d", len(files))
	}
}

func TestListFiles_Subfolder(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "sub/one.md", "1")
	seedFile(t, root, "sub/two.md", "2")
	seedFile(t, root, "root.md", "r")

	files, err := ws.ListFiles("sub")
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "meeting-notes.md", "some content")
	seedFile(t, root, "todo.md", "other content")

	results, err := ws.SearchFiles("meeting", "", SearchModeName)
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "a.md", "the quick brown fox")
	seedFile(t, root, "b.md", "lazy dog")

	results, err := ws.SearchFiles("brown fox", "", SearchModeContent)
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "fox-story.md", "no mention here")
	seedFile(t, root, "other.md", "the quick brown fox")

	results, err := ws.SearchFiles("fox", "", SearchModeBoth)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (name match + content match), got %d", len(results))
	}
}

func TestSearchFiles_CaseInsensitive(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "README.md", "Hello World")

	results, err := ws.SearchFiles("readme", "", SearchModeName)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive name search, got %d", len(results))
	}
}

func TestSearchFiles_NoResults(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "a.md", "hello")

	results, err := ws.SearchFiles("zzzzz", "", SearchModeName)
	if err != nil {
		t.Fatalf("SearchFiles error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// 7. MoveFile
// ---------------------------------------------------------------------------

func TestMoveFile_MovesFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "src.md", "content")

	ok := ws.MoveFile("src.md", "dest.md")
	if !ok {
		t.Fatal("MoveFile returned false")
	}

	if _, err := os.Stat(filepath.Join(root, "src.md")); !os.IsNotExist(err) {
		t.Fatal("source file still exists after move")
	}

	data, err := os.ReadFile(filepath.Join(root, "dest.md"))
	if err != nil {
		t.Fatalf("dest file not found: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestMoveFile_CreatesDestDirs(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "flat.md", "data")

	ok := ws.MoveFile("flat.md", "deep/nested/file.md")
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
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "file.md", "data")

	ok := ws.MoveFile("file.md", "../../escaped.md")
	if ok {
		t.Fatal("MoveFile should block path traversal on destination")
	}

	ok = ws.MoveFile("../../etc/passwd", "stolen.md")
	if ok {
		t.Fatal("MoveFile should block path traversal on source")
	}
}

func TestMoveFile_SourceNotExist(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	ok := ws.MoveFile("nonexistent.md", "dest.md")
	if ok {
		t.Fatal("MoveFile should return false for non-existent source")
	}
}

// ---------------------------------------------------------------------------
// 8. GetStats
// ---------------------------------------------------------------------------

func TestGetStats_CountsFilesAndSizes(t *testing.T) {
	ws, root := newTestWorkspace(t)
	seedFile(t, root, "a.md", "hello")      // 5 bytes
	seedFile(t, root, "b.md", "world")      // 5 bytes
	seedFile(t, root, "sub/c.md", "foobar") // 6 bytes

	stats, err := ws.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}

	if stats.TotalFiles < 3 {
		t.Fatalf("expected at least 3 total files, got %d", stats.TotalFiles)
	}
	if stats.TotalSize != 16 {
		t.Fatalf("expected total size 16, got %d", stats.TotalSize)
	}
	if stats.LastModified == nil {
		t.Fatal("LastModified should not be nil")
	}
}

func TestGetStats_EmptyWorkspace(t *testing.T) {
	ws, _ := newTestWorkspace(t)

	stats, err := ws.GetStats()
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
		t.Fatal("LastModified should be nil for empty workspace")
	}
}

// ---------------------------------------------------------------------------
// 9. Path security
// ---------------------------------------------------------------------------

func TestPathSecurity_SymlinkDetection(t *testing.T) {
	ws, root := newTestWorkspace(t)
	outside := t.TempDir()

	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	_, ok := ws.ReadFile("link.txt")
	if ok {
		t.Fatal("ReadFile should not follow symlinks")
	}

	ok = ws.WriteFile("link.txt", "overwrite")
	if ok {
		t.Fatal("WriteFile should not write through symlinks")
	}
}

func TestPathSecurity_SymlinkDirectory(t *testing.T) {
	ws, root := newTestWorkspace(t)
	outside := t.TempDir()

	link := filepath.Join(root, "linked-dir")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	files, err := ws.ListFiles("")
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
	ws, _ := newTestWorkspace(t)

	traversalPaths := []string{
		"../escape",
		"../../etc/passwd",
		"foo/../../escape",
		"foo/../../../escape",
	}

	for _, p := range traversalPaths {
		_, ok := ws.ReadFile(p)
		if ok {
			t.Fatalf("ReadFile should block traversal path: %s", p)
		}

		ok = ws.WriteFile(p, "bad")
		if ok {
			t.Fatalf("WriteFile should block traversal path: %s", p)
		}
	}
}

func TestPathSecurity_IntermediateSymlink(t *testing.T) {
	ws, root := newTestWorkspace(t)
	outside := t.TempDir()

	dir := filepath.Join(root, "legit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seedFile(t, root, "legit/file.md", "data")

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	_, ok := ws.ReadFile("legit/file.md")
	if ok {
		t.Fatal("ReadFile should detect intermediate symlink segments")
	}
}

// ---------------------------------------------------------------------------
// 10. Stop (no-op)
// ---------------------------------------------------------------------------

func TestStop_NoOp(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	if err := ws.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 11. RootPath
// ---------------------------------------------------------------------------

func TestRootPath_ReturnsAbsolutePath(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if ws.RootPath() != root {
		t.Fatalf("RootPath() = %q; want %q", ws.RootPath(), root)
	}
}
