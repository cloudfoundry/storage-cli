//go:build benchmark

package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	var (
		candidate    = flag.String("candidate", "", "path to candidate binary (required)")
		baseline     = flag.String("baseline", "", "path to baseline binary for comparison (optional)")
		provider     = flag.String("provider", "", "storage provider: s3|gcs|azurebs|alioss (required)")
		configPath   = flag.String("config", "", "path to provider JSON configuration file (required)")
		backendLabel = flag.String("backend-label", "", "backend label used in reports (defaults to the provider name; set explicitly when the backend differs, e.g. -provider s3 -backend-label ceph)")
		profileFlag  = flag.String("profile", "quick", "benchmark profile: quick|full")
		opTimeout    = flag.Duration("timeout", 10*time.Minute, "per-operation timeout (applied to each put and get)")
		namespace    = flag.String("namespace", "benchmark", "object key namespace prefix; scope cleanup to owned keys only")
		outputDir    = flag.String("output", ".", "directory in which to write results.json, results.csv, summary.md, report.html")
		runID        = flag.String("run-id", "", "unique run identifier; defaults to a nanosecond timestamp")
	)
	flag.Parse()

	if *candidate == "" || *provider == "" || *configPath == "" {
		fmt.Fprintln(os.Stderr, "benchmark: -candidate, -provider, and -config are required")
		flag.Usage()
		os.Exit(1)
	}

	if *backendLabel == "" {
		*backendLabel = *provider
	}

	p := Profile(*profileFlag)
	if p != ProfileQuick && p != ProfileFull {
		fmt.Fprintf(os.Stderr, "benchmark: invalid profile %q; must be quick or full\n", *profileFlag)
		os.Exit(1)
	}

	if *runID == "" {
		*runID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	if err := os.MkdirAll(*outputDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	cfg := RunConfig{
		CandidateBinary: *candidate,
		BaselineBinary:  *baseline,
		Provider:        *provider,
		BackendLabel:    *backendLabel,
		ConfigPath:      *configPath,
		Profile:         p,
		OpTimeout:       *opTimeout,
		Namespace:       *namespace,
		RunID:           *runID,
	}

	result := RunBenchmark(cfg)

	if err := WriteJSON(result, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: write results.json: %v\n", err)
		os.Exit(1)
	}
	if err := WriteCSV(result, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: write results.csv: %v\n", err)
		os.Exit(1)
	}

	stats := ComputeRunStats(result)

	if err := WriteSummary(stats, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: write summary.md: %v\n", err)
		os.Exit(1)
	}
	if err := WriteHTMLReport(stats, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark: write report.html: %v\n", err)
		os.Exit(1)
	}

	if result.HasFailures {
		fmt.Fprintln(os.Stderr, "benchmark: run completed with failures; see results.json for details")
		os.Exit(1)
	}

	fmt.Printf("benchmark: run completed successfully; results written to %s\n", *outputDir)
}
