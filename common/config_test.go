package common

import (
	"log/slog"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Storage-CLI Config", func() {
	Context("when not initialized", func() {
		It("IsDebug returns false", func() {
			Expect(IsDebug()).To(BeFalse())
		})

		It("Config is nil", func() {
			Expect(GetConfig()).To(BeNil())
		})
	})

	Context("when initialized with 'debug' level", func() {
		BeforeEach(func() {
			InitConfig(slog.LevelDebug)
		})

		It("IsDebug returns true", func() {
			Expect(IsDebug()).To(BeTrue())
		})

		It("Config is not nil", func() {
			Expect(GetConfig()).ToNot(BeNil())
		})

		AfterEach(func() {
			instance = nil
			once = sync.Once{}
		})
	})

	Context("when initialized with 'info' level", func() {
		BeforeEach(func() {
			InitConfig(slog.LevelInfo)
		})

		It("IsDebug returns false", func() {
			Expect(IsDebug()).To(BeFalse())
		})

		It("Config is not nil", func() {
			Expect(GetConfig()).ToNot(BeNil())
		})

		AfterEach(func() {
			instance = nil
			once = sync.Once{}
		})
	})
})
