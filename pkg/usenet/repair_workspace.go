package usenet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/logger"
)

type RepairWorkspace struct {
	BasePath  string
	NZBID     string
	GroupName string
	Dir       string

	logger zerolog.Logger
}

func NewRepairWorkspace(nzbID, groupName string) *RepairWorkspace {
	cfg := config.Get()
	basePath := cfg.Usenet.RepairWorkPath
	sanitizedGroup := sanitizeName(groupName)
	dir := filepath.Join(basePath, nzbID, sanitizedGroup)

	return &RepairWorkspace{
		BasePath:  basePath,
		NZBID:     nzbID,
		GroupName: groupName,
		Dir:       dir,
		logger:    logger.New("repair-workspace"),
	}
}

func (w *RepairWorkspace) Create() error {
	if err := os.MkdirAll(w.Dir, 0755); err != nil {
		return fmt.Errorf("create repair workspace %s: %w", w.Dir, err)
	}
	w.logger.Info().Str("dir", w.Dir).Msg("Created repair workspace")
	return nil
}

func (w *RepairWorkspace) PayloadDir() string {
	return filepath.Join(w.Dir, "payload")
}

func (w *RepairWorkspace) Par2Dir() string {
	return filepath.Join(w.Dir, "par2")
}

func (w *RepairWorkspace) StagePayloadDir() string {
	dir := w.PayloadDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		w.logger.Warn().Err(err).Str("dir", dir).Msg("Failed to create payload dir")
	}
	return dir
}

func (w *RepairWorkspace) StagePar2Dir() string {
	dir := w.Par2Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		w.logger.Warn().Err(err).Str("dir", dir).Msg("Failed to create par2 dir")
	}
	return dir
}

func (w *RepairWorkspace) PayloadFilePath(fileName string) string {
	return filepath.Join(w.PayloadDir(), sanitizeName(fileName))
}

func (w *RepairWorkspace) Par2FilePath(fileName string) string {
	return filepath.Join(w.Par2Dir(), sanitizeName(fileName))
}

func (w *RepairWorkspace) FindPar2IndexFile() string {
	entries, err := os.ReadDir(w.Par2Dir())
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".par2") && !strings.Contains(name, ".vol") {
			return filepath.Join(w.Par2Dir(), e.Name())
		}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".par2") {
			return filepath.Join(w.Par2Dir(), e.Name())
		}
	}
	return ""
}

func (w *RepairWorkspace) RepairedFilePath(fileName string) string {
	return filepath.Join(w.Dir, "repaired", sanitizeName(fileName))
}

func (w *RepairWorkspace) Cleanup() {
	cfg := config.Get()
	if cfg.Usenet.KeepRepairArtifacts {
		return
	}
	if err := os.RemoveAll(w.Dir); err != nil {
		w.logger.Warn().Err(err).Str("dir", w.Dir).Msg("Failed to clean up repair workspace")
	} else {
		w.logger.Info().Str("dir", w.Dir).Msg("Cleaned up repair workspace")
	}
}

func sanitizeName(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
	return name
}

func FilesExist(paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func NonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}
