package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/storage-cli/dav/app"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

type FakeRunner struct {
	Config       davconf.Config
	SetConfigErr error
	RunArgs      []string
	RunErr       error
}

func (r *FakeRunner) SetConfig(newConfig davconf.Config) (err error) {
	r.Config = newConfig
	return r.SetConfigErr
}

func (r *FakeRunner) Run(cmdArgs []string) (err error) {
	r.RunArgs = cmdArgs
	return r.RunErr
}

func pathToFixture(file string) string {
	pwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	fixturePath := filepath.Join(pwd, "../test_assets", file)

	absPath, err := filepath.Abs(fixturePath)
	Expect(err).ToNot(HaveOccurred())

	return absPath
}

var _ = Describe("App", func() {

	It("reads the CA cert from config", func() {
		configFile, _ := os.Open(pathToFixture("dav-cli-config-with-ca.json")) //nolint:errcheck
		defer configFile.Close()                                               //nolint:errcheck
		davConfig, _ := davconf.NewFromReader(configFile)                      //nolint:errcheck

		runner := &FakeRunner{}
		app := New(runner, davConfig)
		err := app.Put("localFile", "remoteFile")
		Expect(err).ToNot(HaveOccurred())

		expectedConfig := davconf.Config{
			User:     "some user",
			Password: "some pwd",
			Endpoint: "https://example.com/some/endpoint",
			Secret:   "77D47E3A0B0F590B73CF3EBD9BB6761E244F90FA6F28BB39F941B0905789863FBE2861FDFD8195ADC81B72BB5310BC18969BEBBF4656366E7ACD3F0E4186FDDA",
			TLS: davconf.TLS{
				Cert: davconf.Cert{
					CA: "ca-cert",
				},
			},
		}

		Expect(runner.Config).To(Equal(expectedConfig))
		Expect(runner.Config.TLS.Cert.CA).ToNot(BeNil())
	})

	It("returns error if CA Cert is invalid", func() {
		configFile, _ := os.Open(pathToFixture("dav-cli-config-with-ca.json")) //nolint:errcheck
		defer configFile.Close()                                               //nolint:errcheck
		davConfig, _ := davconf.NewFromReader(configFile)                      //nolint:errcheck

		runner := &FakeRunner{
			SetConfigErr: errors.New("invalid cert"),
		}

		app := New(runner, davConfig)
		err := app.Put("localFile", "remoteFile")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Invalid CA Certificate: invalid cert"))

	})

	It("runs the put command", func() {
		configFile, _ := os.Open(pathToFixture("dav-cli-config.json")) //nolint:errcheck
		defer configFile.Close()                                       //nolint:errcheck
		davConfig, _ := davconf.NewFromReader(configFile)              //nolint:errcheck

		runner := &FakeRunner{}

		app := New(runner, davConfig)
		err := app.Put("localFile", "remoteFile")
		Expect(err).ToNot(HaveOccurred())

		expectedConfig := davconf.Config{
			User:     "some user",
			Password: "some pwd",
			Endpoint: "http://example.com/some/endpoint",
			Secret:   "77D47E3A0B0F590B73CF3EBD9BB6761E244F90FA6F28BB39F941B0905789863FBE2861FDFD8195ADC81B72BB5310BC18969BEBBF4656366E7ACD3F0E4186FDDA",
		}

		Expect(runner.Config).To(Equal(expectedConfig))
		Expect(runner.Config.TLS.Cert.CA).To(BeEmpty())
		Expect(runner.RunArgs).To(Equal([]string{"put", "localFile", "remoteFile"}))
	})

	It("returns error from the cmd runner", func() {

		configFile, _ := os.Open(pathToFixture("dav-cli-config.json")) //nolint:errcheck
		defer configFile.Close()                                       //nolint:errcheck
		davConfig, _ := davconf.NewFromReader(configFile)              //nolint:errcheck

		runner := &FakeRunner{
			RunErr: errors.New("fake-run-error"),
		}

		app := New(runner, davConfig)
		err := app.Put("localFile", "remoteFile")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fake-run-error"))
	})

	Context("Checking functionalities", func() {
		// var app *App
		var davConfig davconf.Config
		BeforeEach(func() {

			configFile, _ := os.Open(pathToFixture("dav-cli-config.json")) //nolint:errcheck
			defer configFile.Close()                                       //nolint:errcheck
			davConfig, _ = davconf.NewFromReader(configFile)               //nolint:errcheck
		})

		It("Exists fails", func() {

			runner := &FakeRunner{
				RunErr: errors.New("object does not exist"),
			}
			app := New(runner, davConfig)

			exist, err := app.Exists("someObject") //nolint:errcheck

			Expect(err.Error()).To(ContainSubstring("object does not exist"))
			Expect(exist).To(BeFalse())

		})

		It("Sign Fails", func() {
			runner := &FakeRunner{
				RunErr: errors.New("can't sign"),
			}

			app := New(runner, davConfig)
			signedurl, err := app.Sign("someObject", "SomeObject", time.Second*100)
			Expect(signedurl).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring("can't sign"))

		})

	})

})
