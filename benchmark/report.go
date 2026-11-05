//go:build benchmark

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	texttemplate "text/template"
	"time"
)

// Stats holds summary statistics over a set of float64 samples.
// NoData is set when no samples are available (e.g. all failed).
type Stats struct {
	Count  int     `json:"count"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
	NoData bool    `json:"no_data"`
}

// SizeStats holds computed statistics for one object size within a BinaryResult.
type SizeStats struct {
	SizeBytes     Size   `json:"size_bytes"`
	SizeLabel     string `json:"size_label"`
	Put           Stats  `json:"put"`
	Get           Stats  `json:"get"`
	PutThroughput Stats  `json:"put_throughput"`
	GetThroughput Stats  `json:"get_throughput"`
	FailedCount   int    `json:"failed_count"`
	TimeoutCount  int    `json:"timeout_count"`
}

// BinaryStats holds computed statistics for all sizes of one binary.
type BinaryStats struct {
	Label  string      `json:"label"`
	SHA256 string      `json:"sha256"`
	Sizes  []SizeStats `json:"sizes"`
}

// RunStats holds all computed statistics derived from a RunResult.
type RunStats struct {
	RunID         string       `json:"run_id"`
	Profile       Profile      `json:"profile"`
	BackendLabel  string       `json:"backend_label"`
	Provider      string       `json:"provider"`
	HasFailures   bool         `json:"has_failures"`
	CleanupErrors []string     `json:"cleanup_errors,omitempty"`
	Candidate     BinaryStats  `json:"candidate"`
	Baseline      *BinaryStats `json:"baseline,omitempty"`
	GeneratedAt   time.Time    `json:"generated_at"`
}

// ComputeRunStats derives RunStats from a RunResult without contacting cloud storage.
func ComputeRunStats(r RunResult) RunStats {
	rs := RunStats{
		RunID:         r.RunID,
		Profile:       r.Profile,
		BackendLabel:  r.BackendLabel,
		Provider:      r.Provider,
		HasFailures:   r.HasFailures,
		CleanupErrors: r.CleanupErrors,
		GeneratedAt:   time.Now(),
	}
	rs.Candidate = computeBinaryStats(r.Candidate)
	if r.Baseline != nil {
		bs := computeBinaryStats(*r.Baseline)
		rs.Baseline = &bs
	}
	return rs
}

func computeBinaryStats(br BinaryResult) BinaryStats {
	bs := BinaryStats{Label: br.Label, SHA256: br.SHA256}
	for _, sr := range br.Sizes {
		ss := SizeStats{SizeBytes: sr.SizeBytes, SizeLabel: sr.SizeLabel}

		var putDurs, putThru, getDurs, getThru []float64
		for _, s := range sr.Samples {
			switch s.Status {
			case StatusFailed:
				ss.FailedCount++
				continue
			case StatusTimeout:
				ss.TimeoutCount++
				continue
			}
			switch s.Operation {
			case Put:
				putDurs = append(putDurs, s.DurationMs)
				putThru = append(putThru, s.ThroughputMiBs)
			case Get:
				getDurs = append(getDurs, s.DurationMs)
				getThru = append(getThru, s.ThroughputMiBs)
			}
		}

		ss.Put = computeStats(putDurs)
		ss.PutThroughput = computeStats(putThru)
		ss.Get = computeStats(getDurs)
		ss.GetThroughput = computeStats(getThru)
		bs.Sizes = append(bs.Sizes, ss)
	}
	return bs
}

func computeStats(values []float64) Stats {
	if len(values) == 0 {
		return Stats{NoData: true}
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	return Stats{
		Count:  len(sorted),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Median: medianSorted(sorted),
	}
}

func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// WriteJSON writes the raw RunResult as indented JSON to results.json in outputDir.
func WriteJSON(r RunResult, outputDir string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "results.json"), data, 0600)
}

var summaryTmplText = `# Benchmark Summary: {{ .BackendLabel }}

Run ID: {{ .RunID }}  
Profile: {{ .Profile }}  
Provider: {{ .Provider }}  
Generated: {{ formatTime .GeneratedAt }}  
Failures: {{ .HasFailures }}  

## Candidate (SHA256: {{ truncate .Candidate.SHA256 16 }})

| Size | PUT median ms | PUT MiB/s | GET median ms | GET MiB/s | Samples | Failures |
|------|--------------|-----------|--------------|-----------|---------|----------|
{{ range .Candidate.Sizes -}}
| {{ .SizeLabel }} | {{ fmtF .Put.Median }} | {{ fmtF .PutThroughput.Median }} | {{ fmtF .Get.Median }} | {{ fmtF .GetThroughput.Median }} | {{ .Put.Count }} | {{ add .FailedCount .TimeoutCount }} |
{{ end }}
{{ if .Baseline -}}
## Baseline (SHA256: {{ truncate .Baseline.SHA256 16 }})

| Size | PUT median ms | PUT MiB/s | GET median ms | GET MiB/s |
|------|--------------|-----------|--------------|-----------|
{{ range .Baseline.Sizes -}}
| {{ .SizeLabel }} | {{ fmtF .Put.Median }} | {{ fmtF .PutThroughput.Median }} | {{ fmtF .Get.Median }} | {{ fmtF .GetThroughput.Median }} |
{{ end }}
{{ end -}}
{{ if .CleanupErrors -}}
## Cleanup Errors

{{ range .CleanupErrors -}}
- {{ . }}
{{ end }}
{{ end -}}
`

// WriteSummary writes a Markdown job summary to summary.md in outputDir.
func WriteSummary(rs RunStats, outputDir string) error {
	funcs := texttemplate.FuncMap{
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		"add":        func(a, b int) int { return a + b },
		"fmtF":       func(f float64) string { return fmt.Sprintf("%.1f", f) },
		"formatTime": func(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") },
	}
	tmpl, err := texttemplate.New("summary").Funcs(funcs).Parse(summaryTmplText)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, rs); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "summary.md"), buf.Bytes(), 0600)
}
