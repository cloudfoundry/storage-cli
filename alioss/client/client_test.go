package client_test

import (
	"errors"
	"os"
	"time"

	"github.com/cloudfoundry/storage-cli/alioss/client"
	"github.com/cloudfoundry/storage-cli/alioss/client/clientfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {

	Context("Put", func() {
		It("uploads a file to a blob", func() {
			storageClient := clientfakes.FakeStorageClient{}

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			tmpFile, _ := os.CreateTemp("", "azure-storage-cli-test") //nolint:errcheck

			aliBlobstore.Put(tmpFile.Name(), "destination_object") //nolint:errcheck

			Expect(storageClient.UploadCallCount()).To(Equal(1))
			sourceFilePath, sourceFileMD5, destination := storageClient.UploadArgsForCall(0)

			Expect(sourceFilePath).To(BeAssignableToTypeOf("source/file/path"))
			Expect(sourceFileMD5).To(Equal("1B2M2Y8AsgTpgAmY7PhCfg=="))
			Expect(destination).To(Equal("destination_object"))
		})
	})

	Context("Get", func() {
		It("get blob downloads to a file", func() {
			storageClient := clientfakes.FakeStorageClient{}

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			aliBlobstore.Get("source_object", "destination/file/path") //nolint:errcheck

			Expect(storageClient.DownloadCallCount()).To(Equal(1))
			sourceObject, destinationFilePath := storageClient.DownloadArgsForCall(0)

			Expect(sourceObject).To(Equal("source_object"))
			Expect(destinationFilePath).To(Equal("destination/file/path"))
		})
	})

	Context("Delete", func() {
		It("delete blob deletes the blob", func() {
			storageClient := clientfakes.FakeStorageClient{}

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			aliBlobstore.Delete("blob") //nolint:errcheck

			Expect(storageClient.DeleteCallCount()).To(Equal(1))
			object := storageClient.DeleteArgsForCall(0)

			Expect(object).To(Equal("blob"))
		})
	})

	Context("Exists", func() {
		It("returns blob.Existing on success", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(true, nil)

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			existsState, err := aliBlobstore.Exists("blob")
			Expect(existsState == true).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			object := storageClient.ExistsArgsForCall(0)
			Expect(object).To(Equal("blob"))
		})

		It("returns blob.NotExisting for not existing blobs", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(false, nil)

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			existsState, err := aliBlobstore.Exists("blob")
			Expect(existsState == false).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			object := storageClient.ExistsArgsForCall(0)
			Expect(object).To(Equal("blob"))
		})

		It("returns blob.ExistenceUnknown and an error in case an error occurred", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(false, errors.New("boom"))

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			existsState, err := aliBlobstore.Exists("blob")
			Expect(existsState == false).To(BeTrue())
			Expect(err).To(HaveOccurred())

			object := storageClient.ExistsArgsForCall(0)
			Expect(object).To(Equal("blob"))
		})
	})

	Context("signed url", func() {
		var timeSec time.Duration

		BeforeEach(func() {
			timeSec = 100 * time.Second
		})

		It("returns a signed url for action 'get'", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.SignedUrlGetReturns("https://the-signed-url", nil)

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			url, err := aliBlobstore.Sign("blob", "get", timeSec)
			Expect(url == "https://the-signed-url").To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			object, expiration := storageClient.SignedUrlGetArgsForCall(0)
			Expect(object).To(Equal("blob"))
			Expect(int(expiration)).To(Equal(int(timeSec.Seconds())))
		})

		It("returns a signed url for action 'put'", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.SignedUrlPutReturns("https://the-signed-url", nil)

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			url, err := aliBlobstore.Sign("blob", "put", timeSec)
			Expect(url == "https://the-signed-url").To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			object, expiration := storageClient.SignedUrlPutArgsForCall(0)
			Expect(object).To(Equal("blob"))
			Expect(int(expiration)).To(Equal(int(timeSec.Seconds())))
		})

		It("fails on unknown action", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.SignedUrlGetReturns("", errors.New("boom"))

			aliBlobstore, err := client.New(&storageClient)
			Expect(err).NotTo(HaveOccurred())
			url, err := aliBlobstore.Sign("blob", "unknown", timeSec)
			Expect(url).To(Equal(""))
			Expect(err).To(HaveOccurred())

			Expect(storageClient.SignedUrlGetCallCount()).To(Equal(0))
		})
	})
})
