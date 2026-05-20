package parser

import (
	"errors"
	"testing"

	"github.com/Tensai75/nzbparser"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func makeGroup(name string, ftype storage.NZBFileType, nfiles int) *FileGroup {
	files := make([]nzbparser.NzbFile, nfiles)
	for i := range files {
		files[i] = nzbparser.NzbFile{
			Number:   i + 1,
			Filename: name,
			Segments: nzbparser.NzbSegments{
				{Number: 1, Bytes: 100, Id: "test-id"},
			},
		}
	}
	return &FileGroup{
		BaseName: name,
		Type:     ftype,
		Files:    files,
		Groups:   map[string]struct{}{"alt.binaries.test": {}},
	}
}

func TestHasPayloadAndPar2Groups(t *testing.T) {
	t.Run("both present", func(t *testing.T) {
		groups := map[string]*FileGroup{
			"payload": makeGroup("test.rar", storage.NZBFileTypeRar, 2),
			"par2":    makeGroup("test.par2", storage.NZBFileTypePar2, 1),
		}
		if !HasPayloadAndPar2Groups(groups) {
			t.Fatal("expected true when both payload and PAR2 groups exist")
		}
	})

	t.Run("only payload", func(t *testing.T) {
		groups := map[string]*FileGroup{
			"payload": makeGroup("test.rar", storage.NZBFileTypeRar, 2),
		}
		if HasPayloadAndPar2Groups(groups) {
			t.Fatal("expected false when no PAR2 groups")
		}
	})

	t.Run("only par2", func(t *testing.T) {
		groups := map[string]*FileGroup{
			"par2": makeGroup("test.par2", storage.NZBFileTypePar2, 1),
		}
		if HasPayloadAndPar2Groups(groups) {
			t.Fatal("expected false when no payload groups")
		}
	})

	t.Run("empty files", func(t *testing.T) {
		groups := map[string]*FileGroup{
			"payload": {BaseName: "empty", Type: storage.NZBFileTypeRar, Files: nil},
			"par2":    {BaseName: "empty", Type: storage.NZBFileTypePar2, Files: nil},
		}
		if HasPayloadAndPar2Groups(groups) {
			t.Fatal("expected false when groups have no files")
		}
	})
}

func TestFindPar2Groups(t *testing.T) {
	payload := makeGroup("test.rar", storage.NZBFileTypeRar, 2)
	par2a := makeGroup("test.par2", storage.NZBFileTypePar2, 1)
	par2b := makeGroup("test.vol.par2", storage.NZBFileTypePar2, 1)

	groups := map[string]*FileGroup{
		"payload": payload,
		"par2_a":  par2a,
		"par2_b":  par2b,
	}

	result := FindPar2Groups(groups)
	if len(result) != 2 {
		t.Fatalf("expected 2 PAR2 groups, got %d", len(result))
	}
}

func TestFindPayloadGroup(t *testing.T) {
	t.Run("returns first payload group", func(t *testing.T) {
		payload := makeGroup("test.rar", storage.NZBFileTypeRar, 2)
		par2 := makeGroup("test.par2", storage.NZBFileTypePar2, 1)
		ignore := makeGroup("test.nfo", storage.NZBFileTypeIgnore, 1)

		groups := map[string]*FileGroup{
			"par2":    par2,
			"payload": payload,
			"ignore":  ignore,
		}

		result := FindPayloadGroup(groups)
		if result == nil {
			t.Fatal("expected a payload group, got nil")
		}
		if result.Type != storage.NZBFileTypeRar {
			t.Fatalf("expected RAR type, got %v", result.Type)
		}
	})

	t.Run("no payload group", func(t *testing.T) {
		groups := map[string]*FileGroup{
			"par2": makeGroup("test.par2", storage.NZBFileTypePar2, 1),
		}
		if result := FindPayloadGroup(groups); result != nil {
			t.Fatal("expected nil when no payload group")
		}
	})
}

func TestGroupToStorageFiles(t *testing.T) {
	group := makeGroup("testfile.rar", storage.NZBFileTypeRar, 2)

	files := GroupToStorageFiles(group)
	if len(files) != 2 {
		t.Fatalf("expected 2 storage files, got %d", len(files))
	}

	for i, sf := range files {
		if sf.Name != "testfile.rar" {
			t.Errorf("file %d: expected Name 'testfile.rar', got %q", i, sf.Name)
		}
		if len(sf.Segments) != 1 {
			t.Errorf("file %d: expected 1 segment, got %d", i, len(sf.Segments))
		}
		if sf.Segments[0].MessageID != "test-id" {
			t.Errorf("file %d: expected MessageID 'test-id', got %q", i, sf.Segments[0].MessageID)
		}
		if sf.Segments[0].Bytes != 100 {
			t.Errorf("file %d: expected Bytes 100, got %d", i, sf.Segments[0].Bytes)
		}
	}
}

func TestGroupToStorageFilesEmpty(t *testing.T) {
	group := &FileGroup{
		BaseName: "empty",
		Files:    []nzbparser.NzbFile{},
	}
	files := GroupToStorageFiles(group)
	if len(files) != 0 {
		t.Fatalf("expected 0 files for empty group, got %d", len(files))
	}
}

func TestIsRepairableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"need more data", errors.New("need more data to decode"), true},
		{"unexpected end", errors.New("unexpected end of archive"), true},
		{"corrupt archive", errors.New("corrupt archive"), true},
		{"damaged", errors.New("damaged archive or file"), true},
		{"CRC failed", errors.New("CRC failed"), true},
		{"no PAR2 data", errors.New("archive parsers failed and no PAR2 data available"), true},
		{"unsupported type", errors.New("unsupported file type: unknown"), false},
		{"connection error", errors.New("connection refused"), false},
		{"no valid files", errors.New("no valid files found in NZB"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRepairableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRepairableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
