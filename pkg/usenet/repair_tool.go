package usenet

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/logger"
)

type RepairResult struct {
	Executable string
	Args       []string
	ExitCode   int
	Output     string
	Duration   time.Duration
	Err        error
}

type RepairTool struct {
	logger zerolog.Logger
}

func NewRepairTool() *RepairTool {
	return &RepairTool{
		logger: logger.New("repair-tool"),
	}
}

func (t *RepairTool) Discover() (string, error) {
	cfg := config.Get()
	if cfg.Usenet.Par2Binary != "" {
		path := cfg.Usenet.Par2Binary
		t.logger.Info().Str("path", path).Msg("Using configured PAR2 binary")
		return path, nil
	}

	candidates := []string{"par2", "par2j", "par2repair"}
	for _, name := range candidates {
		path, err := exec.LookPath(name)
		if err == nil {
			t.logger.Info().Str("path", path).Str("name", name).Msg("Found PAR2 binary in PATH")
			return path, nil
		}
	}

	return "", fmt.Errorf("no PAR2 executable found: configure usenet.par2_binary or install one (par2, par2j, par2repair)")
}

type ToolFamily int

const (
	ToolFamilyPar2  ToolFamily = iota
	ToolFamilyPar2j
	ToolFamilyPar2Repair
)

func detectToolFamily(name string) ToolFamily {
	base := strings.ToLower(name)
	if strings.Contains(base, "par2j") {
		return ToolFamilyPar2j
	}
	if strings.Contains(base, "par2repair") {
		return ToolFamilyPar2Repair
	}
	return ToolFamilyPar2
}

func (t *RepairTool) BuildArgs(indexFile string) []string {
	return []string{"repair", indexFile}
}

func (t *RepairTool) Run(ctx context.Context, executable string, args []string, workDir string, timeout time.Duration) *RepairResult {
	result := &RepairResult{
		Executable: executable,
		Args:       args,
	}

	start := time.Now()

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, executable, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.logger.Info().
		Str("executable", executable).
		Strs("args", args).
		Str("workDir", workDir).
		Str("timeout", timeout.String()).
		Msg("Running PAR2 repair")

	err := cmd.Run()

	result.Duration = time.Since(start)
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n" + stderr.String()
		} else {
			output = stderr.String()
		}
	}
	result.Output = output

	if cmdCtx.Err() == context.DeadlineExceeded {
		result.Err = fmt.Errorf("PAR2 repair timed out after %s", timeout)
		result.ExitCode = -1
		t.logger.Error().Err(result.Err).Dur("duration", result.Duration).Msg("PAR2 repair timed out")
		return result
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Err = fmt.Errorf("PAR2 repair failed (exit %d): %s", result.ExitCode, truncateOutput(output, 1024))
		t.logger.Error().
			Int("exitCode", result.ExitCode).
			Dur("duration", result.Duration).
			Msg("PAR2 repair failed")
		return result
	}

	result.ExitCode = 0
	t.logger.Info().
		Int("exitCode", 0).
		Dur("duration", result.Duration).
		Msg("PAR2 repair succeeded")

	return result
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (r *RepairResult) Success() bool {
	return r.Err == nil && r.ExitCode == 0
}

func (r *RepairResult) String() string {
	return fmt.Sprintf("RepairResult{exe=%s, exit=%d, duration=%s, err=%v}",
		r.Executable, r.ExitCode, r.Duration, r.Err)
}
