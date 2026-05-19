package parser

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirrobot01/decypharr/internal/nntp"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

const (
	par2Magic        = "PAR2\000PKT"
	par2FileDescType = "PAR 2.0\000FileDesc"
	par2BlockSize    = 16384
	par2FetchSize    = 256 * 1024
)

type Par2FileDesc struct {
	FileID      [16]byte
	FileHash    [16]byte
	File16kHash [16]byte
	FileLength  uint64
	FileName    string
}

func parsePar2FileDesc(data []byte) []Par2FileDesc {
	var descs []Par2FileDesc
	offset := 0

	for offset < len(data) {
		if offset+8 > len(data) {
			break
		}
		if string(data[offset:offset+8]) != par2Magic {
			offset++
			continue
		}
		if offset+16 > len(data) {
			break
		}
		packetLen := int(binary.LittleEndian.Uint64(data[offset+8 : offset+16]))
		if packetLen < 64 {
			packetLen = len(data) - offset
		}
		if offset+packetLen > len(data) {
			packetLen = len(data) - offset
		}
		if offset+64 > len(data) {
			break
		}
		packetType := string(data[offset+48 : offset+64])
		if packetType == par2FileDescType {
			body := data[offset+64 : offset+packetLen]
			if len(body) < 56 {
				offset += packetLen
				continue
			}
			var fd Par2FileDesc
			copy(fd.FileID[:], body[0:16])
			copy(fd.FileHash[:], body[16:32])
			copy(fd.File16kHash[:], body[32:48])
			fd.FileLength = binary.LittleEndian.Uint64(body[48:56])
			nameBytes := body[56:]
			if idx := bytes.IndexByte(nameBytes, 0); idx >= 0 {
				nameBytes = nameBytes[:idx]
			}
			if len(nameBytes) > 0 {
				fd.FileName = string(nameBytes)
				descs = append(descs, fd)
			}
		}
		offset += packetLen
		if packetLen == 0 {
			break
		}
	}
	return descs
}

func (p *NZBParser) fetchAndParsePar2(ctx context.Context) ([]Par2FileDesc, error) {
	if len(p.par2Files) == 0 {
		return nil, nil
	}

	par2File := &p.par2Files[0]
	if len(par2File.Segments) == 0 {
		return nil, fmt.Errorf("PAR2 file has no segments")
	}

	firstSegment := par2File.Segments[0]
	var data *nntp.YencMetadata
	err := p.manager.ExecuteWithFailover(ctx, func(conn *nntp.Connection) error {
		var e error
		data, e = conn.GetHeaderPrefix(firstSegment.Id, par2FetchSize)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PAR2 data: %w", err)
	}
	if data == nil || len(data.Snippet) == 0 {
		return nil, fmt.Errorf("PAR2 data is empty")
	}

	descs := parsePar2FileDesc(data.Snippet)
	if len(descs) == 0 {
		return nil, fmt.Errorf("no FileDesc entries found in PAR2")
	}
	return descs, nil
}

func (p *NZBParser) deobfuscateGroupWithPar2(ctx context.Context, group *FileGroup, descs []Par2FileDesc) (bool, error) {
	if len(descs) == 0 {
		return false, nil
	}

	renamed := 0
	for i := range group.Files {
		if len(group.Files[i].Segments) == 0 {
			continue
		}

		var data *nntp.YencMetadata
		err := p.manager.ExecuteWithFailover(ctx, func(conn *nntp.Connection) error {
			var e error
			data, e = conn.GetHeaderPrefix(group.Files[i].Segments[0].Id, par2BlockSize)
			return e
		})
		if err != nil || data == nil || len(data.Snippet) == 0 {
			continue
		}

		snippet := data.Snippet
		if len(snippet) > par2BlockSize {
			snippet = snippet[:par2BlockSize]
		}

		hash := md5.Sum(snippet)

		for _, fd := range descs {
			if bytes.Equal(hash[:], fd.File16kHash[:]) {
				group.Files[i].Filename = fd.FileName
				renamed++
				break
			}
		}
	}

	if renamed == 0 {
		return false, nil
	}

	for _, f := range group.Files {
		detected := p.detectFileType(f.Filename)
		if detected == storage.NZBFileTypeRar || detected == storage.NZBFileTypeZip || detected == storage.NZBFileTypeSevenZip {
			group.Type = detected
			break
		}
	}

	firstOrig := ""
	for _, f := range group.Files {
		if f.Filename != "" {
			firstOrig = f.Filename
			break
		}
	}
	if firstOrig != "" {
		group.ActualFilename = firstOrig
		name := strings.TrimSuffix(firstOrig, filepath.Ext(firstOrig))
		if name != "" {
			group.BaseName = name
		}
	}

	return true, nil
}

func (p *NZBParser) par2DeobfuscationAttempt(ctx context.Context, group *FileGroup, password string) ([]*storage.NZBFile, error) {
	if len(p.par2Files) == 0 {
		return nil, fmt.Errorf("archive parsers failed (possibly requires PAR2 repair or unsupported obfuscation)")
	}

	p.logger.Warn().Str("group", group.BaseName).Msg("All archive parsers failed, attempting PAR2 deobfuscation")

	if p.par2Descs == nil {
		descs, err := p.fetchAndParsePar2(ctx)
		if err != nil {
			return nil, fmt.Errorf("archive parsers failed and PAR2 deobfuscation failed: %w", err)
		}
		p.par2Descs = descs
	}

	renamed, err := p.deobfuscateGroupWithPar2(ctx, group, p.par2Descs)
	if err != nil || !renamed {
		if err != nil {
			return nil, fmt.Errorf("archive parsers failed and PAR2 deobfuscation error: %w", err)
		}
		return nil, fmt.Errorf("archive parsers failed and PAR2 deobfuscation could not match any files")
	}

	p.logger.Warn().Str("group", group.BaseName).Str("type", string(group.Type)).Msg("PAR2 deobfuscation successful, retrying archive parsing")
	return p.processFileGroup(ctx, group, password)
}
