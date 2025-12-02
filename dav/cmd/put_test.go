package cmd_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/storage-cli/dav/cmd"
	testcmd "github.com/cloudfoundry/storage-cli/dav/cmd/testing"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

func runPut(config davconf.Config, args []string) error {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	factory := NewFactory(logger)
	factory.SetConfig(config) //nolint:errcheck

	cmd, err := factory.Create("put")
	Expect(err).ToNot(HaveOccurred())

	return cmd.Run(args)
}

func fileBytes(path string) []byte {
	file, err := os.Open(path)
	Expect(err).ToNot(HaveOccurred())

	content, err := io.ReadAll(file)
	Expect(err).ToNot(HaveOccurred())

	return content
}

var _ = Describe("PutCmd", func() {
	Describe("Run", func() {
		var (
			handler        func(http.ResponseWriter, *http.Request)
			config         davconf.Config
			ts             *httptest.Server
			sourceFilePath string
			targetBlob     string
			serverWasHit   bool
		)
		BeforeEach(func() {
			pwd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			sourceFilePath = filepath.Join(pwd, "../test_assets/cat.jpg")
			targetBlob = "some-other-awesome-guid"
			serverWasHit = false

			handler = func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				serverWasHit = true
				req := testcmd.NewHTTPRequest(r)

				username, password, err := req.ExtractBasicAuth()
				Expect(err).ToNot(HaveOccurred())
				Expect(req.URL.Path).To(Equal("/d1/" + targetBlob))
				Expect(req.Method).To(Equal("PUT"))
				Expect(req.ContentLength).To(Equal(int64(1718186)))
				Expect(username).To(Equal("some user"))
				Expect(password).To(Equal("some pwd"))

				expectedBytes := fileBytes(sourceFilePath)
				actualBytes, _ := io.ReadAll(r.Body) //nolint:errcheck
				Expect(expectedBytes).To(Equal(actualBytes))

				w.WriteHeader(201)
			}
		})

		AfterEach(func() {
			defer ts.Close()
		})

		AssertPutBehavior := func() {
			It("uploads the blob with valid args", func() {
				err := runPut(config, []string{sourceFilePath, targetBlob})
				Expect(err).ToNot(HaveOccurred())
				Expect(serverWasHit).To(BeTrue())
			})

			It("returns err with incorrect arg count", func() {
				err := runPut(davconf.Config{}, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Incorrect usage"))
			})
		}

		Context("with http endpoint", func() {
			BeforeEach(func() {
				ts = httptest.NewServer(http.HandlerFunc(handler))
				config = davconf.Config{
					User:     "some user",
					Password: "some pwd",
					Endpoint: ts.URL,
				}

			})

			AssertPutBehavior()
		})

		Context("with https endpoint", func() {
			BeforeEach(func() {
				ts = httptest.NewTLSServer(http.HandlerFunc(handler))

				rootCa, err := testcmd.ExtractRootCa(ts)
				Expect(err).ToNot(HaveOccurred())

				config = davconf.Config{
					User:     "some user",
					Password: "some pwd",
					Endpoint: ts.URL,
					TLS: davconf.TLS{
						Cert: davconf.Cert{
							CA: rootCa,
						},
					},
				}
			})

			AssertPutBehavior()
		})
	})
})
