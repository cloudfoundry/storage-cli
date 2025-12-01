package storage

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Factory", func() {
	Describe("New", func() {

		var configFile *os.File
		BeforeEach(func() {
			configFile, _ = os.CreateTemp("", "some-config-file") //linting:noerrcheck

		})
		AfterEach(func() {
			configFile.Close()
			os.Remove("some-config-file")
		})

		Context("alioss", func() {
			It("Create a client", func() {
				original := newAliossClient
				DeferCleanup(func() {
					newAliossClient = original
				})

				mockClient := &FakeStorager{}
				newAliossClient = func(configFile *os.File) (Storager, error) {
					return mockClient, nil
				}

				client, err := NewStorageClient("alioss", configFile)
				Expect(client).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				Expect(client).To(Equal(mockClient))
			})

		})

		Context("azurebs", func() {
			It("Create a client", func() {
				original := newAzurebsClient
				DeferCleanup(func() {
					newAzurebsClient = original
				})

				mockClient := &FakeStorager{}
				newAzurebsClient = func(configFile *os.File) (Storager, error) {
					return mockClient, nil
				}

				client, err := NewStorageClient("azurebs", configFile)
				Expect(client).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				Expect(client).To(Equal(mockClient))
			})

		})

		Context("dav", func() {
			It("Create a client", func() {
				original := newDavClient
				DeferCleanup(func() {
					newDavClient = original
				})

				mockClient := &FakeStorager{}
				newDavClient = func(configFile *os.File) (Storager, error) {
					return mockClient, nil
				}

				client, err := NewStorageClient("dav", configFile)
				Expect(client).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				Expect(client).To(Equal(mockClient))
			})

		})

		Context("gcs", func() {
			It("Create a client", func() {
				original := newGcsClient
				DeferCleanup(func() {
					newGcsClient = original
				})

				mockClient := &FakeStorager{}
				newGcsClient = func(configFile *os.File) (Storager, error) {
					return mockClient, nil
				}

				client, err := NewStorageClient("gcs", configFile)
				Expect(client).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				Expect(client).To(Equal(mockClient))
			})

		})

		Context("s3", func() {
			It("Create a client", func() {
				original := newS3Client
				DeferCleanup(func() {
					newS3Client = original
				})

				mockClient := &FakeStorager{}
				newS3Client = func(configFile *os.File) (Storager, error) {
					return mockClient, nil
				}

				client, err := NewStorageClient("s3", configFile)
				Expect(client).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				Expect(client).To(Equal(mockClient))
			})

		})

		It("Unimplemented Client", func() {
			client, err := NewStorageClient("random-client", configFile)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
		})
	})
})
