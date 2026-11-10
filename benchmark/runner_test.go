//go:build benchmark

package main

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunBinary", func() {
	var (
		store *mockStore
		cfg   RunConfig
	)

	BeforeEach(func() {
		store = newMockStore(GinkgoT().TempDir())
		cfg = testRunConfig(GinkgoT().Name(), ProfileQuick)
	})

	Context("quick profile", func() {
		It("all samples succeed with correct counts", func() {
			result, cleanupErrs := runBinary(cfg, store.runner(), "", "test-sha", "candidate")

			Expect(cleanupErrs).To(BeEmpty())
			Expect(result.Sizes).To(HaveLen(4))
			for _, sz := range result.Sizes {
				put, get := 0, 0
				for _, s := range sz.Samples {
					Expect(s.Status).To(Equal(StatusOK), "size %s op %s sample %d error: %s",
						sz.SizeLabel, s.Operation, s.SampleIndex, s.Error)
					Expect(s.DurationMs).To(BeNumerically(">", 0), "size %s op %s: non-positive duration", sz.SizeLabel, s.Operation)
					switch s.Operation {
					case Put:
						put++
					case Get:
						get++
					}
				}
				Expect(put).To(Equal(3), "size %s: wrong put count", sz.SizeLabel)
				Expect(get).To(Equal(3), "size %s: wrong get count", sz.SizeLabel)
			}
		})

		It("verifies SHA256 round-trip integrity", func() {
			result, _ := runBinary(cfg, store.runner(), "", "test-sha", "candidate")

			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					if s.Operation == Get && s.Status == StatusFailed {
						Expect(s.Error).To(Equal("skipped: preceding put did not succeed"),
							"size %s get sample %d unexpected failure", sz.SizeLabel, s.SampleIndex)
					}
				}
			}
		})
	})

	Context("full profile", func() {
		BeforeEach(func() {
			cfg = testRunConfig(GinkgoT().Name(), ProfileFull)
		})

		It("has 5 samples per size across 5 sizes", func() {
			result, _ := runBinary(cfg, store.runner(), "", "test-sha", "candidate")

			Expect(result.Sizes).To(HaveLen(5))
			for _, sz := range result.Sizes {
				put := 0
				for _, s := range sz.Samples {
					if s.Operation == Put {
						put++
					}
				}
				Expect(put).To(Equal(5), "size %s: wrong put count", sz.SizeLabel)
			}
		})
	})

	Context("put failure", func() {
		It("marks put and get samples as failed", func() {
			result, _ := runBinary(cfg, failRunner(store, Put), "", "test-sha", "candidate")

			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					Expect(s.Status).To(Equal(StatusFailed),
						"size %s op %s sample %d: expected failed", sz.SizeLabel, s.Operation, s.SampleIndex)
				}
			}
		})
	})

	Context("get failure", func() {
		It("marks get samples failed and put samples ok", func() {
			result, _ := runBinary(cfg, failRunner(store, Get), "", "test-sha", "candidate")

			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					switch s.Operation {
					case Put:
						Expect(s.Status).To(Equal(StatusOK), "size %s put sample %d", sz.SizeLabel, s.SampleIndex)
					case Get:
						Expect(s.Status).To(Equal(StatusFailed), "size %s get sample %d", sz.SizeLabel, s.SampleIndex)
					}
				}
			}
		})
	})

	Context("delete failure", func() {
		It("records cleanup errors without affecting transfer status", func() {
			result, cleanupErrs := runBinary(cfg, deleteFailRunner(store), "", "test-sha", "candidate")

			Expect(cleanupErrs).NotTo(BeEmpty())
			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					Expect(s.Status).To(Equal(StatusOK),
						"size %s op %s sample %d: delete failure should not affect transfer status",
						sz.SizeLabel, s.Operation, s.SampleIndex)
				}
			}
		})
	})

	Context("timeout", func() {
		It("marks put samples as timeout", func() {
			result, _ := runBinary(cfg, timeoutRunner(store, Put), "", "test-sha", "candidate")

			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					if s.Operation == Put {
						Expect(s.Status).To(Equal(StatusTimeout),
							"size %s put sample %d", sz.SizeLabel, s.SampleIndex)
					}
				}
			}
		})
	})

	Context("object key namespacing", func() {
		It("prefixes all keys with namespace and run ID", func() {
			cfg.Namespace = "ns-test"
			cfg.RunID = "run-abc"
			result, _ := runBinary(cfg, store.runner(), "", "test-sha", "candidate")

			prefix := filepath.Join("ns-test", "run-abc")
			for _, sz := range result.Sizes {
				for _, s := range sz.Samples {
					Expect(s.ObjectKey).To(HavePrefix(prefix),
						"object key %q missing namespace prefix", s.ObjectKey)
				}
			}
		})
	})

	Context("with baseline", func() {
		It("runs both candidate and baseline independently", func() {
			baselineStore := newMockStore(GinkgoT().TempDir())

			candidate, candErrs := runBinary(cfg, store.runner(), "", "cand-sha", "candidate")
			baseline, baseErrs := runBinary(cfg, baselineStore.runner(), "", "base-sha", "baseline")

			Expect(candErrs).To(BeEmpty())
			Expect(baseErrs).To(BeEmpty())
			Expect(candidate.Sizes).To(HaveLen(4))
			Expect(baseline.Sizes).To(HaveLen(4))
			Expect(candidate.SHA256).To(Equal("cand-sha"))
			Expect(baseline.SHA256).To(Equal("base-sha"))
		})
	})

})
