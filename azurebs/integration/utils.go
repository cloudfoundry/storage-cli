package integration

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/cloudfoundry/storage-cli/azurebs/config"
)

const alphanum = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GenerateRandomString(params ...int) string {
	size := 25
	if len(params) == 1 {
		size = params[0]
	}

	randBytes := make([]byte, size)
	for i := range randBytes {
		randBytes[i] = alphanum[rand.Intn(len(alphanum))]
	}
	return string(randBytes)
}

func MakeConfigFile(cfg *config.AZStorageConfig) string {
	cfgBytes, err := json.Marshal(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	tmpFile, err := os.CreateTemp("", "azure-storage-cli-test")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	_, err = tmpFile.Write(cfgBytes)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = tmpFile.Close()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return tmpFile.Name()
}

func MakeContentFile(content string) string {
	tmpFile, err := os.CreateTemp("", "azure-storage-test-content")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	_, err = tmpFile.Write([]byte(content))
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	err = tmpFile.Close()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return tmpFile.Name()
}

func RunCli(cliPath string, configPath string, storageType string, subcommand string, args ...string) (*gexec.Session, error) {
	cmdArgs := []string{
		"-c",
		configPath,
		"-s",
		storageType,
		"-log-level",
		"info",
		subcommand,
	}
	cmdArgs = append(cmdArgs, args...)
	command := exec.Command(cliPath, cmdArgs...)
	gexecSession, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return nil, err
	}
	gexecSession.Wait(1 * time.Minute)
	return gexecSession, nil
}
