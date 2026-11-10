//go:build benchmark

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func fixtureResult(withBaseline bool) RunResult {
	putDurs := []float64{100, 200, 300}
	getDurs := []float64{400, 500, 600}

	makeSamples := func() []OperationSample {
		var samples []OperationSample
		for i, d := range putDurs {
			samples = append(samples, OperationSample{
				Operation:      Put,
				SizeBytes:      KiB,
				SampleIndex:    i,
				Status:         StatusOK,
				DurationMs:     d,
				ThroughputMiBs: float64(KiB) / float64(MiB) / (d / 1000),
			})
		}
		for i, d := range getDurs {
			samples = append(samples, OperationSample{
				Operation:      Get,
				SizeBytes:      KiB,
				SampleIndex:    i,
				Status:         StatusOK,
				DurationMs:     d,
				ThroughputMiBs: float64(KiB) / float64(MiB) / (d / 1000),
			})
		}
		return samples
	}

	r := RunResult{
		RunID:        "test-run-1",
		Profile:      ProfileQuick,
		Provider:     "s3",
		BackendLabel: "test-backend",
		Namespace:    "test",
		StartedAt:    time.Now(),
		FinishedAt:   time.Now(),
		Candidate: BinaryResult{
			Label:  "candidate",
			SHA256: "abcdef1234567890",
			Sizes:  []SizeResult{{SizeBytes: KiB, SizeLabel: "1KiB", Samples: makeSamples()}},
		},
	}

	if withBaseline {
		var baseSamples []OperationSample
		for i, d := range putDurs {
			baseSamples = append(baseSamples, OperationSample{
				Operation: Put, SizeBytes: KiB, SampleIndex: i,
				Status: StatusOK, DurationMs: d * 0.9,
			})
		}
		for i, d := range getDurs {
			baseSamples = append(baseSamples, OperationSample{
				Operation: Get, SizeBytes: KiB, SampleIndex: i,
				Status: StatusOK, DurationMs: d * 0.9,
			})
		}
		bl := BinaryResult{
			Label:  "baseline",
			SHA256: "baseline1234567890",
			Sizes:  []SizeResult{{SizeBytes: KiB, SizeLabel: "1KiB", Samples: baseSamples}},
		}
		r.Baseline = &bl
	}

	return r
}

func fixtureResultWithFailure() RunResult {
	r := fixtureResult(false)
	r.Candidate.Sizes[0].Samples = append(r.Candidate.Sizes[0].Samples, OperationSample{
		Operation: Put, SizeBytes: KiB, SampleIndex: 3,
		Status: StatusFailed, Error: "injected failure",
	})
	r.HasFailures = true
	return r
}

var _ = Describe("ComputeStats", func() {
	It("computes median correctly for odd count", func() {
		var s Stats
		s.compute([]float64{300, 100, 200})
		Expect(s.Median).To(Equal(200.0))
		Expect(s.Min).To(Equal(100.0))
		Expect(s.Max).To(Equal(300.0))
		Expect(s.Count).To(Equal(3))
	})

	It("computes median correctly for even count", func() {
		var s Stats
		s.compute([]float64{100, 200, 300, 400})
		Expect(s.Median).To(Equal(250.0))
	})

	It("returns NoData for empty input", func() {
		var s Stats
		s.compute(nil)
		Expect(s.NoData).To(BeTrue())
		Expect(s.Count).To(Equal(0))
	})
})

var _ = Describe("ComputeRunStats", func() {
	It("computes candidate medians correctly", func() {
		rs := ComputeRunStats(fixtureResult(false))

		Expect(rs.Candidate.Sizes).To(HaveLen(1))
		ss := rs.Candidate.Sizes[0]
		Expect(ss.Put.Median).To(Equal(200.0)) // put durations: 100,200,300
		Expect(ss.Get.Median).To(Equal(500.0)) // get durations: 400,500,600
		Expect(ss.Put.FailedCount).To(Equal(0))
	})

	It("excludes failed samples from statistics", func() {
		rs := ComputeRunStats(fixtureResultWithFailure())
		ss := rs.Candidate.Sizes[0]

		Expect(ss.Put.Median).To(Equal(200.0), "failed sample should not skew median")
		Expect(ss.Put.FailedCount).To(Equal(1))
	})

	It("produces nil baseline and no comparisons when no baseline was run", func() {
		rs := ComputeRunStats(fixtureResult(false))
		Expect(rs.Baseline).To(BeNil())
	})

	It("populates baseline stats when baseline was run", func() {
		rs := ComputeRunStats(fixtureResult(true))
		Expect(rs.Baseline).NotTo(BeNil())
		Expect(rs.Baseline.Sizes).To(HaveLen(1))
	})
})

var _ = Describe("WriteJSON", func() {
	It("produces valid parseable JSON with correct run_id", func() {
		outDir := GinkgoT().TempDir()
		r := fixtureResult(false)

		Expect(WriteJSON(r, outDir)).To(Succeed())

		data, err := os.ReadFile(filepath.Join(outDir, "results.json"))
		Expect(err).NotTo(HaveOccurred())

		var parsed RunResult
		Expect(json.Unmarshal(data, &parsed)).To(Succeed())
		Expect(parsed.RunID).To(Equal(r.RunID))
		Expect(parsed.Candidate.Sizes).To(HaveLen(1))
	})
})

var _ = Describe("WriteSummary", func() {
	It("contains expected sections and labels", func() {
		outDir := GinkgoT().TempDir()
		rs := ComputeRunStats(fixtureResult(true))

		Expect(WriteSummary(rs, outDir)).To(Succeed())

		data, err := os.ReadFile(filepath.Join(outDir, "summary.md"))
		Expect(err).NotTo(HaveOccurred())
		content := string(data)

		for _, want := range []string{rs.BackendLabel, rs.RunID, "Candidate", "Baseline", "1KiB"} {
			Expect(content).To(ContainSubstring(want))
		}
	})
})
