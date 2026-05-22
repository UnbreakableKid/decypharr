package parser

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Tensai75/nzbparser"
	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/pkg/storage"
	"github.com/sirrobot01/decypharr/pkg/usenet/types"
)

func TestRARParser_SingleStoredFile(t *testing.T) {
	parser := NewRARParser(nil, 1, zerolog.Nop())

	// Mock parseArchive to return a single stored file
	parser.parseArchiveFn = func(ctx context.Context, volumes []*types.Volume, password string) (*RARArchiveInfo, error) {
		return &RARArchiveInfo{
			Version: RARVersion5,
			Files: []*RARFileEntry{
				{
					Name:             "movie.mkv",
					UncompressedSize: 100,
					PackedSize:       100,
					IsStored:         true,
					VolumeParts: []*types.RARVolumePart{
						{
							Name:         "movie.mkv",
							DataOffset:   0,
							PackedSize:   100,
							UnpackedSize: 100,
							PartNumber:   0,
						},
					},
				},
			},
		}, nil
	}

	first := nzbparser.NzbFile{
		Number:   1,
		Filename: "sample.rar",
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 100, Id: "seg-1"},
		},
	}
	group := &FileGroup{
		BaseName: "sample",
		Files:    []nzbparser.NzbFile{first},
		metadata: &fileAnalysisResult{
			fileSize:     100,
			lastFileSize: 100,
			segmentSize:  100,
		},
	}

	files, err := parser.Process(context.Background(), group, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Name != "movie.mkv" {
		t.Errorf("expected filename movie.mkv, got %q", f.Name)
	}
	if f.FileType != storage.NZBFileTypeMedia {
		t.Errorf("expected file type media, got %v", f.FileType)
	}
	if f.Size != 100 {
		t.Errorf("expected size 100, got %d", f.Size)
	}
	if len(f.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(f.Segments))
	}
	if f.Segments[0].MessageID != "seg-1" {
		t.Errorf("expected message ID seg-1, got %q", f.Segments[0].MessageID)
	}
	if f.Segments[0].SegmentDataStart != 0 {
		t.Errorf("expected SegmentDataStart 0, got %d", f.Segments[0].SegmentDataStart)
	}
}

func TestRARParser_RecursiveExtraction(t *testing.T) {
	parser := NewRARParser(nil, 1, zerolog.Nop())

	// Mock parseArchive to return a nested rar on depth 0, and a media file on depth 1
	parser.parseArchiveFn = func(ctx context.Context, volumes []*types.Volume, password string) (*RARArchiveInfo, error) {
		if len(volumes) == 0 {
			return nil, fmt.Errorf("no volumes")
		}
		volName := volumes[0].Name
		if strings.Contains(volName, "sample.rar") || strings.Contains(volName, "sample") {
			// Depth 0: Contains nested.rar
			return &RARArchiveInfo{
				Version: RARVersion5,
				Files: []*RARFileEntry{
					{
						Name:             "nested.rar",
						UncompressedSize: 100,
						PackedSize:       100,
						IsStored:         true,
						VolumeParts: []*types.RARVolumePart{
							{
								Name:         "nested.rar",
								DataOffset:   0,
								PackedSize:   100,
								UnpackedSize: 100,
								PartNumber:   0,
							},
						},
					},
				},
			}, nil
		} else if strings.Contains(volName, "nested") {
			// Depth 1: Contains video.mkv at data offset 15
			return &RARArchiveInfo{
				Version: RARVersion5,
				Files: []*RARFileEntry{
					{
						Name:             "video.mkv",
						UncompressedSize: 80,
						PackedSize:       80,
						IsStored:         true,
						VolumeParts: []*types.RARVolumePart{
							{
								Name:         "video.mkv",
								DataOffset:   15,
								PackedSize:   80,
								UnpackedSize: 80,
								PartNumber:   0,
							},
						},
					},
				},
			}, nil
		}
		return nil, fmt.Errorf("unexpected volume name: %s", volName)
	}

	first := nzbparser.NzbFile{
		Number:   1,
		Filename: "sample.rar",
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 100, Id: "parent-seg-1"},
		},
	}
	group := &FileGroup{
		BaseName: "sample",
		Files:    []nzbparser.NzbFile{first},
		metadata: &fileAnalysisResult{
			fileSize:     100,
			lastFileSize: 100,
			segmentSize:  100,
		},
	}

	files, err := parser.Process(context.Background(), group, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Name != "video.mkv" {
		t.Errorf("expected filename video.mkv, got %q", f.Name)
	}
	if f.FileType != storage.NZBFileTypeMedia {
		t.Errorf("expected file type media, got %v", f.FileType)
	}
	if f.Size != 80 {
		t.Errorf("expected size 80, got %d", f.Size)
	}
	if len(f.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(f.Segments))
	}
	if f.Segments[0].MessageID != "parent-seg-1" {
		t.Errorf("expected message ID parent-seg-1, got %q", f.Segments[0].MessageID)
	}
	// SegmentDataStart should accumulate: 0 (parent SegmentDataStart) + 15 (nested data offset) = 15
	if f.Segments[0].SegmentDataStart != 15 {
		t.Errorf("expected SegmentDataStart 15, got %d", f.Segments[0].SegmentDataStart)
	}
}

func TestRARParser_RecursionLimit(t *testing.T) {
	parser := NewRARParser(nil, 1, zerolog.Nop())

	// Mock parseArchive to always return a RAR file, causing infinite recursion
	parser.parseArchiveFn = func(ctx context.Context, volumes []*types.Volume, password string) (*RARArchiveInfo, error) {
		if len(volumes) == 0 {
			return nil, fmt.Errorf("no volumes")
		}
		// Extract base name of the volume to avoid infinite string build
		volName := volumes[0].Name
		return &RARArchiveInfo{
			Version: RARVersion5,
			Files: []*RARFileEntry{
				{
					Name:             volName + "_nested.rar",
					UncompressedSize: 100,
					PackedSize:       100,
					IsStored:         true,
					VolumeParts: []*types.RARVolumePart{
						{
							Name:         volName + "_nested.rar",
							DataOffset:   0,
							PackedSize:   100,
							UnpackedSize: 100,
							PartNumber:   0,
						},
					},
				},
			},
		}, nil
	}

	first := nzbparser.NzbFile{
		Number:   1,
		Filename: "sample.rar",
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 100, Id: "seg-1"},
		},
	}
	group := &FileGroup{
		BaseName: "sample",
		Files:    []nzbparser.NzbFile{first},
		metadata: &fileAnalysisResult{
			fileSize:     100,
			lastFileSize: 100,
			segmentSize:  100,
		},
	}

	_, err := parser.Process(context.Background(), group, "")
	if err == nil {
		t.Fatal("expected error due to recursion limit, got nil")
	}

	if !strings.Contains(err.Error(), "recursion depth limit exceeded") {
		t.Errorf("expected recursion depth limit error, got: %v", err)
	}
}
