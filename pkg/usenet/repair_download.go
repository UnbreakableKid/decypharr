package usenet

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/logger"
	"github.com/sirrobot01/decypharr/internal/nntp"
	"github.com/sirrobot01/decypharr/pkg/storage"
	"github.com/sourcegraph/conc/pool"
)

type FileStager struct {
	client *nntp.Client
	logger zerolog.Logger
}

func NewFileStager(client *nntp.Client) *FileStager {
	return &FileStager{
		client: client,
		logger: logger.New("file-stager"),
	}
}

func (s *FileStager) StageFile(ctx context.Context, file *storage.NZBFile, destPath string) error {
	if err := os.MkdirAll(dirFromPath(destPath), 0755); err != nil {
		return fmt.Errorf("create staging dir for %s: %w", file.Name, err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create staging file %s: %w", destPath, err)
	}
	defer f.Close()

	if len(file.Segments) == 0 {
		return fmt.Errorf("file %s has no segments", file.Name)
	}

	totalSize := file.Size
	if totalSize <= 0 {
		for _, seg := range file.Segments {
			end := seg.EndOffset
			if seg.SegmentDataStart > 0 {
				end -= seg.SegmentDataStart
			}
			if end > totalSize {
				totalSize = end
			}
		}
	}

	if err := s.downloadSegments(ctx, file.Segments, totalSize, f); err != nil {
		return fmt.Errorf("download segments for %s: %w", file.Name, err)
	}

	s.logger.Info().
		Str("file", file.Name).
		Str("dest", destPath).
		Int64("size", totalSize).
		Msg("Staged file to disk")

	return nil
}

func (s *FileStager) downloadSegments(ctx context.Context, segments []storage.NZBSegment, totalSize int64, writer io.Writer) error {
	type segResult struct {
		index int
		data  []byte
		err   error
	}

	results := make(chan segResult, len(segments))

	p := pool.New().WithContext(ctx).WithMaxGoroutines(10)

	for idx := range segments {
		segIdx := idx
		seg := segments[idx]

		p.Go(func(ctx context.Context) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var data []byte
			err := s.client.ExecuteWithFailover(ctx, func(conn *nntp.Connection) error {
				d, e := conn.GetDecodedBody(seg.MessageID)
				data = d
				return e
			})
			if err != nil {
				results <- segResult{index: segIdx, err: fmt.Errorf("segment %d: %w", segIdx, err)}
				return nil
			}
			if seg.SegmentDataStart > 0 {
				if seg.SegmentDataStart >= int64(len(data)) {
					results <- segResult{index: segIdx, err: fmt.Errorf("segment %d: SegmentDataStart exceeds data", segIdx)}
					return nil
				}
				data = data[seg.SegmentDataStart:]
			}
			if int64(len(data)) > seg.Bytes && seg.Bytes > 0 {
				data = data[:seg.Bytes]
			}
			results <- segResult{index: segIdx, data: data}
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		close(results)
		return err
	}
	close(results)

	orderedResults := make([][]byte, len(segments))
	for r := range results {
		if r.err != nil {
			return r.err
		}
		orderedResults[r.index] = r.data
	}

	for _, data := range orderedResults {
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("write segment data: %w", err)
		}
	}

	return nil
}

func dirFromPath(path string) string {
	idx := len(path)
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			idx = i
			break
		}
	}
	if idx == len(path) {
		return "."
	}
	return path[:idx]
}
