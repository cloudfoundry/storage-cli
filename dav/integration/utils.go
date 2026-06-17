package integration

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/cloudfoundry/storage-cli/dav/config"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const alphaNum = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenerateRandomString(params ...int) string {
	size := 25
	if len(params) == 1 {
		size = params[0]
	}
	randBytes := make([]byte, size)
	for i := range randBytes {
		randBytes[i] = alphaNum[rand.Intn(len(alphaNum))]
	}
	return string(randBytes)
}

func MakeConfigFile(cfg *config.Config) string {
	cfgBytes, err := json.Marshal(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	tmpFile, err := os.CreateTemp("", "davcli-test")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	_, err = tmpFile.Write(cfgBytes)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = tmpFile.Close()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return tmpFile.Name()
}

func MakeContentFile(content string) string {
	tmpFile, err := os.CreateTemp("", "davcli-test-content")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	_, err = tmpFile.Write([]byte(content))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = tmpFile.Close()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return tmpFile.Name()
}

func RunCli(cliPath string, configPath string, storageType string, subcommand string, args ...string) (*gexec.Session, error) {
	cmdArgs := []string{"-c", configPath, "-s", storageType, subcommand}
	cmdArgs = append(cmdArgs, args...)
	command := exec.Command(cliPath, cmdArgs...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return nil, err
	}
	session.Wait(1 * time.Minute)
	return session, nil
}
