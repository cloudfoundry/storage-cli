//go:build benchmark

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// WriteCSV writes one row per OperationSample to results.csv in outputDir.
func WriteCSV(r RunResult, outputDir string) error {
	f, err := os.Create(filepath.Join(outputDir, "results.csv"))
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"run_id", "backend_label", "provider", "binary_label",
		"size_label", "size_bytes", "operation",
		"sample_index", "status",
		"duration_ms", "throughput_mibs", "object_key", "error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	binaries := []BinaryResult{r.Candidate}
	if r.Baseline != nil {
		binaries = append(binaries, *r.Baseline)
	}
	for _, bin := range binaries {
		for _, sz := range bin.Sizes {
			for _, s := range sz.Samples {
				row := []string{
					r.RunID,
					r.BackendLabel,
					r.Provider,
					bin.Label,
					sz.SizeLabel,
					fmt.Sprintf("%d", sz.SizeBytes),
					string(s.Operation),
					fmt.Sprintf("%d", s.SampleIndex),
					string(s.Status),
					fmt.Sprintf("%.3f", s.DurationMs),
					fmt.Sprintf("%.3f", s.ThroughputMiBs),
					s.ObjectKey,
					s.Error,
				}
				if err := w.Write(row); err != nil {
					return err
				}
			}
		}
	}
	return nil
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

// htmlSizeRow is the per-size data passed to the HTML template.
type htmlSizeRow struct {
	SizeLabel    string
	Chart        htmltemplate.HTML
	CandPutMs    float64
	CandGetMs    float64
	CandPutMiBs  float64
	CandGetMiBs  float64
	BasePutMs    float64
	BaseGetMs    float64
	BasePutMiBs  float64
	BaseGetMiBs  float64
	HasBaseline  bool
	FailedCount  int
	TimeoutCount int
}

type htmlReportData struct {
	BackendLabel  string
	RunID         string
	Profile       Profile
	Provider      string
	GeneratedAt   string
	HasFailures   bool
	CleanupErrors []string
	HasBaseline   bool
	CandSHA256    string
	BaseSHA256    string
	Sizes         []htmlSizeRow
}

var htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Benchmark Report: {{.BackendLabel}}</title>
  <style>
    body{font-family:monospace;margin:2em;color:#222}
    h1,h2,h3{color:#333}
    table{border-collapse:collapse;margin:.5em 0}
    th,td{border:1px solid #ccc;padding:.3em .7em;text-align:right}
    th{background:#f5f5f5;text-align:center}
    td:first-child{text-align:left}
    .fail{color:#c00;font-weight:bold}
    .ok{color:#2a7a2a}
    .size-section{margin-bottom:2.5em;border-top:2px solid #ddd;padding-top:1em}
  </style>
</head>
<body>
<h1>Benchmark Report: {{.BackendLabel}}</h1>
<p>Run ID: {{.RunID}} &nbsp;|&nbsp; Profile: {{.Profile}} &nbsp;|&nbsp; Provider: {{.Provider}}</p>
<p>Generated: {{.GeneratedAt}}</p>
{{if .HasFailures}}<p class="fail">⚠ Run completed with failures — see table below</p>{{else}}<p class="ok">✓ All measured samples succeeded</p>{{end}}
<p>Candidate SHA256: <code>{{.CandSHA256}}</code></p>
{{if .HasBaseline}}<p>Baseline SHA256: <code>{{.BaseSHA256}}</code></p>{{end}}

{{range .Sizes}}
<div class="size-section">
  <h2>{{.SizeLabel}}</h2>
  {{.Chart}}
  <table>
    <tr>
      <th>Binary</th><th>PUT median (ms)</th><th>PUT (MiB/s)</th>
      <th>GET median (ms)</th><th>GET (MiB/s)</th>
      <th>Failures</th>
    </tr>
    <tr>
      <td>Candidate</td>
      <td>{{printf "%.1f" .CandPutMs}}</td><td>{{printf "%.2f" .CandPutMiBs}}</td>
      <td>{{printf "%.1f" .CandGetMs}}</td><td>{{printf "%.2f" .CandGetMiBs}}</td>
      <td>{{add .FailedCount .TimeoutCount}}</td>
    </tr>
    {{if .HasBaseline}}
    <tr>
      <td>Baseline</td>
      <td>{{printf "%.1f" .BasePutMs}}</td><td>{{printf "%.2f" .BasePutMiBs}}</td>
      <td>{{printf "%.1f" .BaseGetMs}}</td><td>{{printf "%.2f" .BaseGetMiBs}}</td>
      <td>—</td>
    </tr>
    {{end}}
  </table>
</div>
{{end}}
{{if .CleanupErrors}}
<h2>Cleanup Errors</h2>
<ul>{{range .CleanupErrors}}<li>{{.}}</li>{{end}}</ul>
{{end}}
</body>
</html>
`

// WriteHTMLReport generates an HTML report with inline SVG bar charts to
// report.html in outputDir. It derives the report from RunStats without
// contacting cloud storage.
func WriteHTMLReport(rs RunStats, outputDir string) error {
	data := htmlReportData{
		BackendLabel:  rs.BackendLabel,
		RunID:         rs.RunID,
		Profile:       rs.Profile,
		Provider:      rs.Provider,
		GeneratedAt:   rs.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		HasFailures:   rs.HasFailures,
		CleanupErrors: rs.CleanupErrors,
		HasBaseline:   rs.Baseline != nil,
		CandSHA256:    rs.Candidate.SHA256,
	}
	if rs.Baseline != nil {
		data.BaseSHA256 = rs.Baseline.SHA256
	}

	for i, cs := range rs.Candidate.Sizes {
		row := htmlSizeRow{
			SizeLabel:    cs.SizeLabel,
			CandPutMs:    cs.Put.Median,
			CandGetMs:    cs.Get.Median,
			CandPutMiBs:  cs.PutThroughput.Median,
			CandGetMiBs:  cs.GetThroughput.Median,
			FailedCount:  cs.FailedCount,
			TimeoutCount: cs.TimeoutCount,
		}
		if rs.Baseline != nil && i < len(rs.Baseline.Sizes) {
			bss := rs.Baseline.Sizes[i]
			row.HasBaseline = true
			row.BasePutMs = bss.Put.Median
			row.BaseGetMs = bss.Get.Median
			row.BasePutMiBs = bss.PutThroughput.Median
			row.BaseGetMiBs = bss.GetThroughput.Median
		}
		row.Chart = htmltemplate.HTML(buildBarChartSVG(row)) //nolint:gosec // derived from numeric data, no user input
		data.Sizes = append(data.Sizes, row)
	}

	funcs := htmltemplate.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tmpl, err := htmltemplate.New("report").Funcs(funcs).Parse(htmlTmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "report.html"), buf.Bytes(), 0600)
}

// buildBarChartSVG generates a horizontal bar chart SVG string for one size row.
// Bars represent PUT and GET median durations for candidate (and baseline if present).
func buildBarChartSVG(row htmlSizeRow) string {
	const (
		labelW    = 140 // left-side operation label area
		chartW    = 460 // bar drawing area
		valueW    = 90  // fixed right-side value label area
		svgWidth  = labelW + chartW + valueW
		barH      = 22
		barGap    = 8
		marginTop = 6
	)

	type barDef struct {
		label string
		value float64
		color string
	}

	bars := []barDef{
		{"Candidate PUT", row.CandPutMs, "#4a90d9"},
		{"Candidate GET", row.CandGetMs, "#5da85d"},
	}
	if row.HasBaseline {
		bars = append(bars,
			barDef{"Baseline PUT", row.BasePutMs, "#9b4fd9"},
			barDef{"Baseline GET", row.BaseGetMs, "#d9784a"},
		)
	}

	maxVal := 0.0
	for _, b := range bars {
		if b.value > maxVal {
			maxVal = b.value
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	totalH := marginTop + len(bars)*(barH+barGap)
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg width="%d" height="%d" xmlns="http://www.w3.org/2000/svg">`, svgWidth, totalH)

	for i, b := range bars {
		y := marginTop + i*(barH+barGap)
		w := int(b.value / maxVal * chartW)
		if w < 2 && b.value > 0 {
			w = 2
		}
		// Operation label on the left.
		fmt.Fprintf(&sb,
			`<text x="%d" y="%d" text-anchor="end" font-family="monospace" font-size="12" fill="#444">%s</text>`,
			labelW-4, y+barH-6, b.label,
		)
		// Bar.
		fmt.Fprintf(&sb,
			`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="3"/>`,
			labelW, y, w, barH, b.color,
		)
		// Value label at fixed position after chart area — never overflows.
		fmt.Fprintf(&sb,
			`<text x="%d" y="%d" font-family="monospace" font-size="11" fill="#555">%.1f ms</text>`,
			labelW+chartW+5, y+barH-6, b.value,
		)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}
