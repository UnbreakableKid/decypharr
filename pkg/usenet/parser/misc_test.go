package parser

import (
	"testing"

	"github.com/Tensai75/nzbparser"
)

func TestGetNZBSegmentsUsesPerFileMetadata(t *testing.T) {
	first := nzbparser.NzbFile{
		Number: 1,
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 100, Id: "first-1"},
			{Number: 2, Bytes: 60, Id: "first-2"},
		},
	}
	second := nzbparser.NzbFile{
		Number: 2,
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 90, Id: "second-1"},
			{Number: 2, Bytes: 90, Id: "second-2"},
		},
	}

	group := &FileGroup{
		BaseName: "sample",
		Files:    []nzbparser.NzbFile{first, second},
		metadata: &fileAnalysisResult{
			fileSize:     160,
			lastFileSize: 160,
			segmentSize:  100,
		},
		fileMeta: map[string]filePartMeta{
			fileMetaKey(first):  {fileSize: 160, segmentSize: 100},
			fileMetaKey(second): {fileSize: 150, segmentSize: 80},
		},
	}

	total, segments := getNZBSegments(1, second, group)
	if total != 150 {
		t.Fatalf("expected total size 150, got %d", total)
	}
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].Bytes != 80 {
		t.Fatalf("expected first segment size 80, got %d", segments[0].Bytes)
	}
	if segments[1].Bytes != 70 {
		t.Fatalf("expected second segment size 70, got %d", segments[1].Bytes)
	}
	if segments[1].StartOffset != 80 || segments[1].EndOffset != 149 {
		t.Fatalf("expected second segment offsets 80-149, got %d-%d", segments[1].StartOffset, segments[1].EndOffset)
	}
}

func TestBuildBaseSegmentsUsesPerFileMetadata(t *testing.T) {
	first := nzbparser.NzbFile{
		Number:   1,
		Filename: "sample.7z.001",
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 100, Id: "first-1"},
			{Number: 2, Bytes: 60, Id: "first-2"},
		},
	}
	second := nzbparser.NzbFile{
		Number:   2,
		Filename: "sample.7z.002",
		Segments: nzbparser.NzbSegments{
			{Number: 1, Bytes: 90, Id: "second-1"},
			{Number: 2, Bytes: 90, Id: "second-2"},
		},
	}

	group := &FileGroup{
		BaseName: "sample",
		Files:    []nzbparser.NzbFile{first, second},
		metadata: &fileAnalysisResult{
			fileSize:     160,
			lastFileSize: 160,
			segmentSize:  100,
		},
		fileMeta: map[string]filePartMeta{
			fileMetaKey(first):  {fileSize: 160, segmentSize: 100},
			fileMetaKey(second): {fileSize: 150, segmentSize: 80},
		},
	}

	baseSegments, volumeInfos, total := buildBaseSegments(group)
	if total != 310 {
		t.Fatalf("expected total concatenated size 310, got %d", total)
	}
	if len(baseSegments) != 4 {
		t.Fatalf("expected 4 base segments, got %d", len(baseSegments))
	}
	if len(volumeInfos) != 2 {
		t.Fatalf("expected 2 volume infos, got %d", len(volumeInfos))
	}
	if volumeInfos[0].Size != 160 {
		t.Fatalf("expected first volume size 160, got %d", volumeInfos[0].Size)
	}
	if volumeInfos[1].Size != 150 {
		t.Fatalf("expected second volume size 150, got %d", volumeInfos[1].Size)
	}
}
