package cmd_test

import (
	"bytes"
	"io"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/storage-cli/dav/cmd"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

func runSign(config davconf.Config, args []string) error {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	factory := NewFactory(logger)
	factory.SetConfig(config) //nolint:errcheck

	cmd, err := factory.Create("sign")
	Expect(err).ToNot(HaveOccurred())

	return cmd.Run(args)
}

var _ = Describe("SignCmd", func() {
	var (
		objectID = "0ca907f2-dde8-4413-a304-9076c9d0978b"
		config   davconf.Config
	)

	It("with valid args", func() {
		old := os.Stdout     // keep backup of the real stdout
		r, w, _ := os.Pipe() //nolint:errcheck
		os.Stdout = w

		err := runSign(config, []string{objectID, "get", "15m"})

		outC := make(chan string)
		// copy the output in a separate goroutine so printing can't block indefinitely
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r) //nolint:errcheck
			outC <- buf.String()
		}()

		// back to normal state
		w.Close()       //nolint:errcheck
		os.Stdout = old // restoring the real stdout
		out := <-outC

		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(HavePrefix("signed/"))
		Expect(out).To(ContainSubstring(objectID))
		Expect(out).To(ContainSubstring("?e="))
		Expect(out).To(ContainSubstring("&st="))
		Expect(out).To(ContainSubstring("&ts="))
	})

	It("returns err with incorrect arg count", func() {
		err := runSign(davconf.Config{}, []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("incorrect usage"))
	})

	It("returns err with non-implemented action", func() {
		err := runSign(davconf.Config{}, []string{objectID, "delete", "15m"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("action not implemented"))
	})

	It("returns err with incorrect duration", func() {
		err := runSign(davconf.Config{}, []string{objectID, "put", "15"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("expiration should be a duration value"))
	})
})
