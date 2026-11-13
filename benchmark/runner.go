//go:build benchmark

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Profile selects the set of object sizes and sample counts for a run.
type Profile string

const (
	ProfileQuick Profile = "quick"
	ProfileFull  Profile = "full"
)

// Size represents an object size in bytes.
type Size int64

const (
	KiB Size = 1 * 1024
	MiB Size = 1 * 1024 * 1024
	GiB Size = 1 * 1024 * 1024 * 1024
)

// OperationStatus records whether a timed operation succeeded.
type OperationStatus string

const (
	StatusOK      OperationStatus = "ok"
	StatusTimeout OperationStatus = "timeout"
	StatusFailed  OperationStatus = "failed"
)

// OperationType identifies a storage CLI operation.
type OperationType string

const (
	Put    OperationType = "put"
	Get    OperationType = "get"
	Delete OperationType = "delete"
)

// OperationSample holds timing and status for one timed put or get invocation.
type OperationSample struct {
	Operation      OperationType   `json:"operation"`
	SizeBytes      Size            `json:"size_bytes"`
	SampleIndex    int             `json:"sample_index"` // 0-based
	Status         OperationStatus `json:"status"`
	DurationMs     float64         `json:"duration_ms"`
	ThroughputMiBs float64         `json:"throughput_mibs"` // 0 when status != ok
	Error          string          `json:"error,omitempty"`
	ObjectKey      string          `json:"object_key"`
}

// SizeResult holds all samples for one object size.
type SizeResult struct {
	SizeBytes Size              `json:"size_bytes"`
	SizeLabel string            `json:"size_label"`
	Samples   []OperationSample `json:"samples"`
}

// BinaryResult holds all size results for one binary under test.
type BinaryResult struct {
	Label      string       `json:"label"` // "candidate" | "baseline"
	BinaryPath string       `json:"binary_path"`
	SHA256     string       `json:"sha256"`
	Sizes      []SizeResult `json:"sizes"`
}

// RunResult is the complete output of a benchmark run.
type RunResult struct {
	RunID         string        `json:"run_id"`
	Profile       Profile       `json:"profile"`
	Provider      string        `json:"provider"`
	BackendLabel  string        `json:"backend_label"`
	Namespace     string        `json:"namespace"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	Candidate     BinaryResult  `json:"candidate"`
	Baseline      *BinaryResult `json:"baseline,omitempty"`
	CleanupErrors []string      `json:"cleanup_errors,omitempty"`
	HasFailures   bool          `json:"has_failures"`
}

// RunConfig holds all parameters for a benchmark run.
type RunConfig struct {
	CandidateBinary string
	BaselineBinary  string // empty = no baseline
	Provider        string
	BackendLabel    string
	ConfigPath      string
	Profile         Profile
	OpTimeout       time.Duration
	Namespace       string
	RunID           string
}

// CLIRunner executes one storage operation and returns timing results.
// For Put: key is the remote object key, localFile is the source file path.
// For Get: key is the remote object key, localFile is the destination file path.
// For Delete: key is the remote object key, localFile is unused.
type CLIRunner func(ctx context.Context, op OperationType, key, localFile string) (elapsed time.Duration, timedOut bool, err error)

// newCLIRunner returns a CLIRunner that invokes the real storage-cli binary.
func newCLIRunner(binaryPath, configPath, provider string) CLIRunner {
	return func(ctx context.Context, op OperationType, key, localFile string) (time.Duration, bool, error) {
		var args []string
		switch op {
		case Put:
			args = []string{string(Put), localFile, key}
		case Get:
			args = []string{string(Get), key, localFile}
		default: // Delete
			args = []string{string(Delete), key}
		}

		cmdArgs := append([]string{"-c", configPath, "-s", provider}, args...)
		cmd := exec.CommandContext(ctx, binaryPath, cmdArgs...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		start := time.Now()
		runErr := cmd.Run()
		elapsed := time.Since(start)

		if ctx.Err() != nil {
			return elapsed, true, fmt.Errorf("operation timed out")
		}

		if runErr != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail != "" {
				return elapsed, false, fmt.Errorf("%w: %s", runErr, detail)
			}
			return elapsed, false, runErr
		}

		return elapsed, false, runErr
	}
}

// profileSizes returns object sizes for the given profile.
func profileSizes(p Profile) []Size {
	quick := []Size{1 * KiB, 1 * MiB, 10 * MiB, 128 * MiB}
	if p == ProfileFull {
		return append(quick, 1*GiB)
	}
	return quick
}

// profileSampleCount returns the number of measured round trips per size.
func profileSampleCount(p Profile) int {
	if p == ProfileFull {
		return 5
	}
	return 3
}

// labelForSize returns a human-readable label for a size.
func labelForSize(s Size) string {
	switch {
	case s < MiB:
		return fmt.Sprintf("%dKiB", s/KiB)
	case s < GiB:
		return fmt.Sprintf("%dMiB", s/MiB)
	default:
		return fmt.Sprintf("%dGiB", s/GiB)
	}
}

// fileSHA256 returns the hex-encoded SHA-256 of the file at path.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// generatePayload writes n deterministic pseudo-random bytes to path.
// Uses a fixed seed derived from the object size so the same size always
// produces the same content, enabling reproducible round-trip integrity checks.
func generatePayload(path string, n Size) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	rng := rand.New(rand.NewPCG(uint64(n), 0)) //nolint:gosec
	buf := make([]byte, 32*KiB)
	written := Size(0)
	for written < n {
		toWrite := Size(len(buf))
		if n-written < toWrite {
			toWrite = n - written
		}
		chunk := buf[:toWrite]
		i := 0
		for ; i+8 <= len(chunk); i += 8 {
			v := rng.Uint64()
			chunk[i] = byte(v)
			chunk[i+1] = byte(v >> 8)
			chunk[i+2] = byte(v >> 16)
			chunk[i+3] = byte(v >> 24)
			chunk[i+4] = byte(v >> 32)
			chunk[i+5] = byte(v >> 40)
			chunk[i+6] = byte(v >> 48)
			chunk[i+7] = byte(v >> 56)
		}
		if i < len(chunk) {
			v := rng.Uint64()
			for ; i < len(chunk); i++ {
				chunk[i] = byte(v)
				v >>= 8
			}
		}
		if _, err := f.Write(chunk); err != nil {
			return err
		}
		written += toWrite
	}
	return nil
}

// objectKey returns the remote key for a specific sample, namespaced to avoid
// collisions across runs.
func objectKey(namespace, runID, provider, label, sizeLabel string, idx int) string {
	return filepath.Join(namespace, runID, provider, label, sizeLabel, fmt.Sprintf("sample-%d", idx))
}

// cleanup deletes a remote key using the runner with a 30-second timeout.
// A separate short timeout ensures a hung delete does not stall the run.
func cleanup(runner CLIRunner, key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := runner(ctx, Delete, key, "")
	return err
}

// runBinary measures all configured sizes using the provided runner and returns
// the result together with any cleanup errors. Cleanup errors are reported
// separately so a slow delete does not mask the original transfer status.
func runBinary(cfg RunConfig, runner CLIRunner, binaryPath, binarySHA, label string) (BinaryResult, []string) {
	result := BinaryResult{
		Label:      label,
		BinaryPath: binaryPath,
		SHA256:     binarySHA,
	}

	sampleCount := profileSampleCount(cfg.Profile)
	var cleanupErrors []string

	for _, sz := range profileSizes(cfg.Profile) {
		szLabel := labelForSize(sz)
		sr := SizeResult{SizeBytes: sz, SizeLabel: szLabel}

		// Generate source payload outside the timer.
		srcFile, err := os.CreateTemp("", "bm-src-*")
		if err != nil {
			sr.Samples = append(sr.Samples, failedSample(Put, sz, 0, "", fmt.Sprintf("create temp: %v", err)))
			result.Sizes = append(result.Sizes, sr)
			continue
		}
		srcFile.Close() //nolint:errcheck

		if err := generatePayload(srcFile.Name(), sz); err != nil {
			os.Remove(srcFile.Name()) //nolint:errcheck
			sr.Samples = append(sr.Samples, failedSample(Put, sz, 0, "", fmt.Sprintf("generate payload: %v", err)))
			result.Sizes = append(result.Sizes, sr)
			continue
		}

		// Hash source before timing; used for round-trip integrity verification.
		srcHash, err := fileSHA256(srcFile.Name())
		if err != nil {
			os.Remove(srcFile.Name()) //nolint:errcheck
			sr.Samples = append(sr.Samples, failedSample(Put, sz, 0, "", fmt.Sprintf("hash source: %v", err)))
			result.Sizes = append(result.Sizes, sr)
			continue
		}

		for i := 0; i < sampleCount; i++ {
			key := objectKey(cfg.Namespace, cfg.RunID, cfg.Provider, label, szLabel, i)

			// --- Timed PUT ---
			putCtx, putCancel := context.WithTimeout(context.Background(), cfg.OpTimeout)
			putDur, putTimedOut, putErr := runner(putCtx, Put, key, srcFile.Name())
			putCancel()

			putSample := OperationSample{
				Operation:   Put,
				SizeBytes:   sz,
				SampleIndex: i,
				ObjectKey:   key,
			}
			if putTimedOut {
				putSample.Status = StatusTimeout
				putSample.Error = "operation timed out"
				putSample.DurationMs = durationMs(putDur)
			} else if putErr != nil {
				putSample.Status = StatusFailed
				putSample.Error = putErr.Error()
				putSample.DurationMs = durationMs(putDur)
			} else {
				putSample.Status = StatusOK
				putSample.DurationMs = durationMs(putDur)
				putSample.ThroughputMiBs = throughputMiBs(sz, putDur)
			}
			sr.Samples = append(sr.Samples, putSample)

			if putSample.Status != StatusOK {
				getSample := OperationSample{
					Operation:   Get,
					SizeBytes:   sz,
					SampleIndex: i,
					ObjectKey:   key,
					Status:      StatusFailed,
					Error:       "skipped: preceding put did not succeed",
				}
				sr.Samples = append(sr.Samples, getSample)
				continue
			}

			// --- Timed GET ---
			dstFile, err := os.CreateTemp("", "bm-dst-*")
			if err != nil {
				sr.Samples = append(sr.Samples, failedSample(Get, sz, i, key, fmt.Sprintf("create download temp: %v", err)))
				if cerr := cleanup(runner, key); cerr != nil {
					cleanupErrors = append(cleanupErrors, fmt.Sprintf("cleanup %s: %v", key, cerr))
				}
				continue
			}
			dstFile.Close()           //nolint:errcheck
			os.Remove(dstFile.Name()) //nolint:errcheck // runner creates the destination file itself

			getCtx, getCancel := context.WithTimeout(context.Background(), cfg.OpTimeout)
			getDur, getTimedOut, getErr := runner(getCtx, Get, key, dstFile.Name())
			getCancel()

			getSample := OperationSample{
				Operation:   Get,
				SizeBytes:   sz,
				SampleIndex: i,
				ObjectKey:   key,
			}
			if getTimedOut {
				getSample.Status = StatusTimeout
				getSample.Error = "operation timed out"
				getSample.DurationMs = durationMs(getDur)
			} else if getErr != nil {
				getSample.Status = StatusFailed
				getSample.Error = getErr.Error()
				getSample.DurationMs = durationMs(getDur)
			} else {
				getSample.DurationMs = durationMs(getDur)
				// Verify integrity outside the timer.
				dstHash, hashErr := fileSHA256(dstFile.Name())
				if hashErr != nil {
					getSample.Status = StatusFailed
					getSample.Error = fmt.Sprintf("hash download: %v", hashErr)
				} else if dstHash != srcHash {
					getSample.Status = StatusFailed
					getSample.Error = fmt.Sprintf("integrity mismatch: got %s want %s", dstHash, srcHash)
				} else {
					getSample.Status = StatusOK
					getSample.ThroughputMiBs = throughputMiBs(sz, getDur)
				}
			}
			os.Remove(dstFile.Name()) //nolint:errcheck
			sr.Samples = append(sr.Samples, getSample)

			if cerr := cleanup(runner, key); cerr != nil {
				cleanupErrors = append(cleanupErrors, fmt.Sprintf("cleanup %s: %v", key, cerr))
			}
		}

		os.Remove(srcFile.Name()) //nolint:errcheck
		result.Sizes = append(result.Sizes, sr)
	}

	return result, cleanupErrors
}

// RunBenchmark creates CLI runners for the configured binaries and measures
// transfer performance. Transfer and cleanup failures both set HasFailures.
func RunBenchmark(cfg RunConfig) RunResult {
	result := RunResult{
		RunID:        cfg.RunID,
		Profile:      cfg.Profile,
		Provider:     cfg.Provider,
		BackendLabel: cfg.BackendLabel,
		Namespace:    cfg.Namespace,
		StartedAt:    time.Now(),
	}

	candidateSHA, err := fileSHA256(cfg.CandidateBinary)
	if err != nil {
		candidateSHA = fmt.Sprintf("(error: %v)", err)
	}
	candidate, cleanupErrs := runBinary(cfg, newCLIRunner(cfg.CandidateBinary, cfg.ConfigPath, cfg.Provider), cfg.CandidateBinary, candidateSHA, "candidate")
	result.Candidate = candidate
	result.CleanupErrors = append(result.CleanupErrors, cleanupErrs...)

	if cfg.BaselineBinary != "" {
		baselineSHA, err := fileSHA256(cfg.BaselineBinary)
		if err != nil {
			baselineSHA = fmt.Sprintf("(error: %v)", err)
		}
		baseline, errs := runBinary(cfg, newCLIRunner(cfg.BaselineBinary, cfg.ConfigPath, cfg.Provider), cfg.BaselineBinary, baselineSHA, "baseline")
		result.Baseline = &baseline
		result.CleanupErrors = append(result.CleanupErrors, errs...)
	}

	result.FinishedAt = time.Now()
	result.HasFailures = hasAnyFailures(result)
	return result
}

// hasAnyFailures returns true when any sample failed or timed out,
// or when any cleanup error was recorded.
func hasAnyFailures(r RunResult) bool {
	for _, sz := range r.Candidate.Sizes {
		for _, s := range sz.Samples {
			if s.Status != StatusOK {
				return true
			}
		}
	}
	if r.Baseline != nil {
		for _, sz := range r.Baseline.Sizes {
			for _, s := range sz.Samples {
				if s.Status != StatusOK {
					return true
				}
			}
		}
	}
	return len(r.CleanupErrors) > 0
}

// failedSample constructs a failed OperationSample without running any CLI.
func failedSample(op OperationType, sz Size, idx int, key, errMsg string) OperationSample {
	return OperationSample{
		Operation:   op,
		SizeBytes:   sz,
		SampleIndex: idx,
		Status:      StatusFailed,
		Error:       errMsg,
		ObjectKey:   key,
	}
}

// durationMs converts a time.Duration to milliseconds as a float64.
func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// throughputMiBs computes MiB/s for a transfer of size bytes over duration d.
func throughputMiBs(size Size, d time.Duration) float64 {
	seconds := float64(d) / float64(time.Second)
	if seconds <= 0 {
		return 0
	}
	return float64(size) / float64(MiB) / seconds
}
