package integration_test

import (
	"os"

	"github.com/cloudfoundry/storage-cli/s3/config"
	"github.com/cloudfoundry/storage-cli/s3/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing AWS assume role ", func() {
	Context("with AWS ASSUME ROLE configurations", func() {
		It("get file from assumed role", func() {
			storageType := "s3"
			accessKeyID := os.Getenv("ACCESS_KEY_ID")
			Expect(accessKeyID).ToNot(BeEmpty(), "ACCESS_KEY_ID must be set")

			secretAccessKey := os.Getenv("SECRET_ACCESS_KEY")
			Expect(secretAccessKey).ToNot(BeEmpty(), "SECRET_ACCESS_KEY must be set")

			assumeRoleArn := os.Getenv("ASSUME_ROLE_ARN")
			Expect(assumeRoleArn).ToNot(BeEmpty(), "ASSUME_ROLE_ARN must be set")

			bucketName := "bosh-s3cli-assume-role-integration-test"
			region := "us-east-1"

			nonAssumedRoleCfg := &config.S3Cli{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				BucketName:      bucketName,
				Region:          region,
				UseSSL:          true,
			}

			assumedRoleCfg := &config.S3Cli{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				BucketName:      bucketName,
				Region:          region,
				AssumeRoleArn:   assumeRoleArn,
				UseSSL:          true,
			}
			s3Filename := "test-file"

			notAssumeRoleConfigPath := integration.MakeConfigFile(nonAssumedRoleCfg)
			defer os.Remove(notAssumeRoleConfigPath) //nolint:errcheck

			s3CLISession, err := integration.RunS3CLI(s3CLIPath, notAssumeRoleConfigPath, storageType, "exists", s3Filename)
			GinkgoWriter.Println("error is %v", err)
			Expect(err).ToNot(HaveOccurred())
			Expect(s3CLISession.ExitCode()).ToNot(BeZero())

			assumeRoleConfigPath := integration.MakeConfigFile(assumedRoleCfg)
			defer os.Remove(assumeRoleConfigPath) //nolint:errcheck

			s3CLISession, err = integration.RunS3CLI(s3CLIPath, assumeRoleConfigPath, storageType, "exists", s3Filename)
			GinkgoWriter.Println("error is %v", err)
			Expect(err).ToNot(HaveOccurred())
			Expect(s3CLISession.ExitCode()).To(BeZero())
		})
	})
})
