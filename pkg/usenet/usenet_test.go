package usenet

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

func TestCheckNZBAvailability(t *testing.T) {
	// Setup a temporary config path so config.Get() doesn't fail
	config.SetConfigPath(t.TempDir())

	// Setup a basic Usenet struct with a dummy logger
	u := &Usenet{
		logger: zerolog.Nop(),
	}

	t.Run("All files succeed", func(t *testing.T) {
		nzb := &storage.NZB{
			ID:        "nzb1",
			TotalSize: 300,
			Files: []storage.NZBFile{
				{Name: "file1.mkv", Size: 100, Segments: []storage.NZBSegment{{Bytes: 100}}, FileType: storage.NZBFileTypeMedia},
				{Name: "file2.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia},
			},
		}

		u.checkFileAvailabilityFunc = func(ctx context.Context, file *storage.NZBFile, samplePercent int) error {
			return nil
		}

		err := u.checkNZBAvailability(context.Background(), nzb)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if nzb.TotalSize != 300 {
			t.Errorf("expected total size to be 300, got %d", nzb.TotalSize)
		}
		if nzb.Files[0].IsDeleted || nzb.Files[1].IsDeleted {
			t.Error("expected files not to be marked as deleted")
		}
	})

	t.Run("One of two files fails availability", func(t *testing.T) {
		nzb := &storage.NZB{
			ID:        "nzb2",
			TotalSize: 300,
			Files: []storage.NZBFile{
				{Name: "file1.mkv", Size: 100, Segments: []storage.NZBSegment{{Bytes: 100}}, FileType: storage.NZBFileTypeMedia},
				{Name: "file2.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia},
			},
		}

		u.checkFileAvailabilityFunc = func(ctx context.Context, file *storage.NZBFile, samplePercent int) error {
			if file.Name == "file1.mkv" {
				return errors.New("not found")
			}
			return nil
		}

		err := u.checkNZBAvailability(context.Background(), nzb)
		if err != nil {
			t.Fatalf("expected no error (partial success should be allowed), got: %v", err)
		}

		if nzb.TotalSize != 200 {
			t.Errorf("expected total size to be 200, got %d", nzb.TotalSize)
		}
		if !nzb.Files[0].IsDeleted {
			t.Error("expected file1.mkv to be marked as deleted")
		}
		if nzb.Files[1].IsDeleted {
			t.Error("expected file2.mkv NOT to be marked as deleted")
		}
	})

	t.Run("All files fail availability", func(t *testing.T) {
		nzb := &storage.NZB{
			ID:        "nzb3",
			TotalSize: 300,
			Files: []storage.NZBFile{
				{Name: "file1.mkv", Size: 100, Segments: []storage.NZBSegment{{Bytes: 100}}, FileType: storage.NZBFileTypeMedia},
				{Name: "file2.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia},
			},
		}

		u.checkFileAvailabilityFunc = func(ctx context.Context, file *storage.NZBFile, samplePercent int) error {
			return errors.New("not found")
		}

		err := u.checkNZBAvailability(context.Background(), nzb)
		if err == nil {
			t.Fatal("expected error since all files failed availability")
		}

		if nzb.TotalSize != 0 {
			t.Errorf("expected total size to be 0, got %d", nzb.TotalSize)
		}
		if !nzb.Files[0].IsDeleted || !nzb.Files[1].IsDeleted {
			t.Error("expected both files to be marked as deleted")
		}
	})

	t.Run("Ignore and deleted files are skipped", func(t *testing.T) {
		nzb := &storage.NZB{
			ID:        "nzb4",
			TotalSize: 500,
			Files: []storage.NZBFile{
				{Name: "file1.par2", Size: 100, Segments: []storage.NZBSegment{{Bytes: 100}}, FileType: storage.NZBFileTypePar2},
				{Name: "file2.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia},
				{Name: "file3.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia, IsDeleted: true},
			},
		}

		u.checkFileAvailabilityFunc = func(ctx context.Context, file *storage.NZBFile, samplePercent int) error {
			if file.Name == "file2.mkv" {
				return errors.New("not found")
			}
			return nil
		}

		err := u.checkNZBAvailability(context.Background(), nzb)
		// Since the only non-deleted playable file (file2.mkv) failed, the overall check should fail.
		if err == nil {
			t.Fatal("expected error since the only playable file failed")
		}
	})

	t.Run("AllowPartialProcess is false and one file fails", func(t *testing.T) {
		// Mock config
		orig := config.Get().Usenet.AllowPartialProcess
		defer func() {
			config.Get().Usenet.AllowPartialProcess = orig
		}()
		allowPartial := false
		config.Get().Usenet.AllowPartialProcess = &allowPartial

		nzb := &storage.NZB{
			ID:        "nzb5",
			TotalSize: 300,
			Files: []storage.NZBFile{
				{Name: "file1.mkv", Size: 100, Segments: []storage.NZBSegment{{Bytes: 100}}, FileType: storage.NZBFileTypeMedia},
				{Name: "file2.mkv", Size: 200, Segments: []storage.NZBSegment{{Bytes: 200}}, FileType: storage.NZBFileTypeMedia},
			},
		}

		u.checkFileAvailabilityFunc = func(ctx context.Context, file *storage.NZBFile, samplePercent int) error {
			if file.Name == "file1.mkv" {
				return errors.New("not found")
			}
			return nil
		}

		err := u.checkNZBAvailability(context.Background(), nzb)
		if err == nil {
			t.Fatal("expected error since allowPartialProcess is false and file1.mkv failed availability")
		}

		if err.Error() != "not found" {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})
}
