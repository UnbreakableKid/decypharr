package parser

import (
	"sort"
	"strings"

	"github.com/Tensai75/nzbparser"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func IsRepairableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "need more data") ||
		strings.Contains(msg, "unexpected end") ||
		strings.Contains(msg, "corrupt") ||
		strings.Contains(msg, "damaged") ||
		strings.Contains(msg, "CRC failed") ||
		strings.Contains(msg, "archive parsers failed and no PAR2 data available")
}

func HasPayloadAndPar2Groups(groups map[string]*FileGroup) bool {
	hasPayload := false
	hasPar2 := false
	for _, g := range groups {
		if g.Type == storage.NZBFileTypePar2 && len(g.Files) > 0 {
			hasPar2 = true
		} else if g.Type != storage.NZBFileTypeIgnore && len(g.Files) > 0 {
			hasPayload = true
		}
	}
	return hasPayload && hasPar2
}

func FindPar2Groups(groups map[string]*FileGroup) []*FileGroup {
	var par2Groups []*FileGroup
	for _, g := range groups {
		if g.Type == storage.NZBFileTypePar2 && len(g.Files) > 0 {
			par2Groups = append(par2Groups, g)
		}
	}
	return par2Groups
}

func FindPayloadGroup(groups map[string]*FileGroup) *FileGroup {
	for _, g := range groups {
		if g.Type != storage.NZBFileTypePar2 && g.Type != storage.NZBFileTypeIgnore && len(g.Files) > 0 {
			return g
		}
	}
	return nil
}

func GroupToStorageFiles(group *FileGroup) []*storage.NZBFile {
	var files []*storage.NZBFile
	for _, nzbFile := range group.Files {
		sf := &storage.NZBFile{
			Name:   nzbFile.Filename,
			Size:   nzbFile.Bytes,
			Groups: nzbFile.Groups,
		}
		sf.Segments = nzbFileToStorageSegments(nzbFile)
		files = append(files, sf)
	}
	return files
}

func nzbFileToStorageSegments(file nzbparser.NzbFile) []storage.NZBSegment {
	if len(file.Segments) == 0 {
		return nil
	}

	sort.Slice(file.Segments, func(i, j int) bool {
		return file.Segments[i].Number < file.Segments[j].Number
	})

	segments := make([]storage.NZBSegment, 0, len(file.Segments))
	currentOffset := int64(0)

	for _, seg := range file.Segments {
		segSize := int64(seg.Bytes)
		storageSeg := storage.NZBSegment{
			Number:      seg.Number,
			MessageID:   seg.Id,
			Bytes:       segSize,
			StartOffset: currentOffset,
			EndOffset:   currentOffset + segSize - 1,
		}
		segments = append(segments, storageSeg)
		currentOffset += segSize
	}
	return segments
}
