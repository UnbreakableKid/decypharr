package usenet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

var testConfigDir string

func TestMain(m *testing.M) {
	var err error
	testConfigDir, err = os.MkdirTemp("", "decypharr-test-*")
	if err != nil {
		os.Stderr.WriteString("Failed to create test dir: " + err.Error() + "\n")
		os.Exit(1)
	}
	config.SetConfigPath(testConfigDir)
	code := m.Run()
	os.RemoveAll(testConfigDir)
	os.Exit(code)
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple.txt", "simple.txt"},
		{"path/to/file.txt", "file.txt"},
		{"file:with*chars?.txt", "file_with_chars_.txt"},
		{"/absolute/path", "path"},
		{"just-name.rar", "just-name.rar"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewRepairWorkspace(t *testing.T) {
	nzbID := "test-nzb-123"
	groupName := "testfile.part01.rar"
	w := NewRepairWorkspace(nzbID, groupName)

	if w.NZBID != nzbID {
		t.Fatalf("expected NZBID %q, got %q", nzbID, w.NZBID)
	}
	if w.GroupName != groupName {
		t.Fatalf("expected GroupName %q, got %q", groupName, w.GroupName)
	}
	if !strings.Contains(w.Dir, nzbID) {
		t.Fatalf("expected Dir to contain NZBID %q, got %q", nzbID, w.Dir)
	}
}

func TestRepairWorkspacePaths(t *testing.T) {
	w := NewRepairWorkspace("nzb1", "group1")
	if !strings.HasSuffix(w.PayloadDir(), "payload") {
		t.Errorf("PayloadDir should end with 'payload', got %q", w.PayloadDir())
	}
	if !strings.HasSuffix(w.Par2Dir(), "par2") {
		t.Errorf("Par2Dir should end with 'par2', got %q", w.Par2Dir())
	}
	if !strings.HasSuffix(w.PayloadFilePath("test.rar"), "test.rar") {
		t.Errorf("PayloadFilePath should end with filename, got %q", w.PayloadFilePath("test.rar"))
	}
	if !strings.HasSuffix(w.Par2FilePath("test.par2"), "test.par2") {
		t.Errorf("Par2FilePath should end with filename, got %q", w.Par2FilePath("test.par2"))
	}
}

func TestRepairWorkspaceCreateAndCleanup(t *testing.T) {
	w := NewRepairWorkspace("test-cleanup", "test-group")
	if err := w.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	defer os.RemoveAll(w.Dir)

	if _, err := os.Stat(w.Dir); os.IsNotExist(err) {
		t.Fatal("workspace dir does not exist after Create")
	}

	payloadDir := w.StagePayloadDir()
	if _, err := os.Stat(payloadDir); os.IsNotExist(err) {
		t.Fatal("payload dir does not exist after StagePayloadDir")
	}

	par2Dir := w.StagePar2Dir()
	if _, err := os.Stat(par2Dir); os.IsNotExist(err) {
		t.Fatal("par2 dir does not exist after StagePar2Dir")
	}
}

func TestFindPar2IndexFile(t *testing.T) {
	w := NewRepairWorkspace("test-index", "test-group")
	if err := w.Create(); err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	defer os.RemoveAll(w.Dir)

	par2Dir := w.StagePar2Dir()

	indexFile := filepath.Join(par2Dir, "testfile.par2")
	if err := os.WriteFile(indexFile, []byte("fake par2 data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	volFile := filepath.Join(par2Dir, "testfile.vol000+01.par2")
	os.WriteFile(volFile, []byte("fake vol data"), 0644)

	result := w.FindPar2IndexFile()
	if result != indexFile {
		t.Fatalf("expected index file %q, got %q", indexFile, result)
	}
}

func TestFilesExist(t *testing.T) {
	dir := t.TempDir()

	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	os.WriteFile(path1, []byte("a"), 0644)
	os.WriteFile(path2, []byte("b"), 0644)

	if !FilesExist([]string{path1, path2}) {
		t.Fatal("expected FilesExist to return true for existing files")
	}

	if FilesExist([]string{path1, filepath.Join(dir, "missing.txt")}) {
		t.Fatal("expected FilesExist to return false when one file is missing")
	}
}

func TestNonEmptyFile(t *testing.T) {
	dir := t.TempDir()

	emptyPath := filepath.Join(dir, "empty.txt")
	os.WriteFile(emptyPath, []byte{}, 0644)
	if NonEmptyFile(emptyPath) {
		t.Fatal("expected NonEmptyFile to return false for empty file")
	}

	nonEmptyPath := filepath.Join(dir, "data.txt")
	os.WriteFile(nonEmptyPath, []byte("data"), 0644)
	if !NonEmptyFile(nonEmptyPath) {
		t.Fatal("expected NonEmptyFile to return true for non-empty file")
	}

	if NonEmptyFile(filepath.Join(dir, "missing.txt")) {
		t.Fatal("expected NonEmptyFile to return false for missing file")
	}
}
