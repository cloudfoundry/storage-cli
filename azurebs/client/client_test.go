package client_test

import (
	"bytes"
	"errors"
	"os"
	"runtime"

	"github.com/cloudfoundry/storage-cli/azurebs/client"
	"github.com/cloudfoundry/storage-cli/azurebs/client/clientfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {

	Context("Put", func() {
		It("uploads a file to a blob", func() {
			storageClient := clientfakes.FakeStorageClient{}

			azBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			file, _ := os.CreateTemp("", "tmpfile") //nolint:errcheck

			azBlobstore.Put(file.Name(), "target/blob") //nolint:errcheck

			Expect(storageClient.UploadCallCount()).To(Equal(1))
			source, dest := storageClient.UploadArgsForCall(0)

			Expect(source).To(BeAssignableToTypeOf((*os.File)(nil)))
			Expect(dest).To(Equal("target/blob"))
		})

		It("uploads a file with UploadStream", func() {
			storageClient := clientfakes.FakeStorageClient{}

			azBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			file, _ := os.CreateTemp("", "tmpfile-test-upload") //nolint:errcheck
			defer os.Remove(file.Name())                        //nolint:errcheck

			contentSize := 1024 * 1024 * 64 // 64MB

			content := bytes.Repeat([]byte("x"), contentSize)
			_, _ = file.Write(content) //nolint:errcheck

			azBlobstore.Put(file.Name(), "target/blob") //nolint:errcheck

			Expect(storageClient.UploadStreamCallCount()).To(Equal(1))
			source, dest := storageClient.UploadStreamArgsForCall(0)

			Expect(source).To(BeAssignableToTypeOf((*os.File)(nil)))
			Expect(dest).To(Equal("target/blob"))
		})

		It("skips the upload if the md5 cannot be calculated from the file", func() {
			storageClient := clientfakes.FakeStorageClient{}

			azBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			err = azBlobstore.Put("the/path", "target/blob")

			Expect(storageClient.UploadCallCount()).To(Equal(0))
			var expectedError string
			if runtime.GOOS == "windows" {
				expectedError = "open the/path: The system cannot find the path specified."
			} else {
				expectedError = "open the/path: no such file or directory"
			}
			Expect(err.Error()).To(Equal(expectedError))
		})

		It("fails if the source file md5 does not match the responded md5", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.UploadReturns([]byte{1, 2, 3}, nil)

			azBlobstore, err := client.New(&storageClient)
			Expect(err).ToNot(HaveOccurred())

			file, _ := os.CreateTemp("", "tmpfile") //nolint:errcheck

			putError := azBlobstore.Put(file.Name(), "target/blob")
			Expect(putError.Error()).To(Equal("MD5 mismatch: expected d41d8cd98f00b204e9800998ecf8427e, got 010203"))

			Expect(storageClient.UploadCallCount()).To(Equal(1))
			source, dest := storageClient.UploadArgsForCall(0)
			Expect(source).To(BeAssignableToTypeOf((*os.File)(nil)))
			Expect(dest).To(Equal("target/blob"))

			Expect(storageClient.DeleteCallCount()).To(Equal(1))
			dest = storageClient.DeleteArgsForCall(0)
			Expect(dest).To(Equal("target/blob"))
		})
	})

	It("get blob downloads to a file", func() {
		storageClient := clientfakes.FakeStorageClient{}

		azBlobstore, err := client.New(&storageClient)
		Expect(err).ToNot(HaveOccurred())

		dstFileName := "tmp-dest-azurebs-get"
		defer os.Remove("tmp-dest-azurebs-get") //nolint:errcheck

		azBlobstore.Get("source/blob", dstFileName) //nolint:errcheck

		Expect(storageClient.DownloadCallCount()).To(Equal(1))

		source, dest := storageClient.DownloadArgsForCall(0)
		Expect(source).To(Equal("source/blob"))
		Expect(dest.Name()).To(Equal(dstFileName))
	})

	It("delete blob deletes the blob", func() {
		storageClient := clientfakes.FakeStorageClient{}

		azBlobstore, err := client.New(&storageClient)
		Expect(err).ToNot(HaveOccurred())

		azBlobstore.Delete("blob") //nolint:errcheck

		Expect(storageClient.DeleteCallCount()).To(Equal(1))
		dest := storageClient.DeleteArgsForCall(0)

		Expect(dest).To(Equal("blob"))
	})

	Context("if the blob existence is checked", func() {
		It("returns blob.Existing on success", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(true, nil)

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			existsState, err := azBlobstore.Exists("blob")
			Expect(existsState == true).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			dest := storageClient.ExistsArgsForCall(0)
			Expect(dest).To(Equal("blob"))
		})

		It("returns blob.NotExisting for not existing blobs", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(false, nil)

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			existsState, err := azBlobstore.Exists("blob")
			Expect(existsState == false).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			dest := storageClient.ExistsArgsForCall(0)
			Expect(dest).To(Equal("blob"))
		})

		It("returns blob.ExistenceUnknown and an error in case an error occurred", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ExistsReturns(false, errors.New("boom"))

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			existsState, err := azBlobstore.Exists("blob")
			Expect(existsState == false).To(BeTrue())
			Expect(err).To(HaveOccurred())

			dest := storageClient.ExistsArgsForCall(0)
			Expect(dest).To(Equal("blob"))
		})
	})

	Context("signed url", func() {
		It("returns a signed url for action 'get'", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.SignedUrlReturns("https://the-signed-url", nil)

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			url, err := azBlobstore.Sign("blob", "get", 100)
			Expect(url == "https://the-signed-url").To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			action, dest, expiration := storageClient.SignedUrlArgsForCall(0)
			Expect(action).To(Equal("GET"))
			Expect(dest).To(Equal("blob"))
			Expect(int(expiration)).To(Equal(100))
		})

		It("fails on unknown action", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.SignedUrlReturns("", errors.New("boom"))

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			url, err := azBlobstore.Sign("blob", "unknown", 100)
			Expect(url).To(Equal(""))
			Expect(err).To(HaveOccurred())

			Expect(storageClient.SignedUrlCallCount()).To(Equal(0))
		})
	})

	Context("list", func() {
		It("lists blobs in a container", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ListReturns([]string{"blob1", "blob2"}, nil)

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			blobs, err := azBlobstore.List("")
			Expect(blobs).To(Equal([]string{"blob1", "blob2"}))
			Expect(err).ToNot(HaveOccurred())

			containerName := storageClient.ListArgsForCall(0)
			Expect(containerName).To(Equal(""))
		})

		It("lists blobs with a prefix in a container", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ListReturns([]string{"pre-blob1", "pre-blob2"}, nil)

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			blobs, err := azBlobstore.List("pre-")
			Expect(blobs).To(Equal([]string{"pre-blob1", "pre-blob2"}))
			Expect(err).ToNot(HaveOccurred())

			containerName := storageClient.ListArgsForCall(0)
			Expect(containerName).To(Equal("pre-"))
		})

		It("returns an error if listing fails", func() {
			storageClient := clientfakes.FakeStorageClient{}
			storageClient.ListReturns(nil, errors.New("boom"))

			azBlobstore, _ := client.New(&storageClient) //nolint:errcheck
			blobs, err := azBlobstore.List("container")
			Expect(blobs).To(BeNil())
			Expect(err).To(HaveOccurred())

			containerName := storageClient.ListArgsForCall(0)
			Expect(containerName).To(Equal("container"))
		})
	})

})
