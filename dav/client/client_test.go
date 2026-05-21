package client_test

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudfoundry/storage-cli/dav/client"
	"github.com/cloudfoundry/storage-cli/dav/client/clientfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {

	Context("Put", func() {
		It("uploads a file to a blob", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.PutReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)

			file, err := os.CreateTemp("", "tmpfile")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(file.Name()) //nolint:errcheck

			_, err = file.WriteString("test content")
			Expect(err).NotTo(HaveOccurred())
			file.Close() //nolint:errcheck

			err = davBlobstore.Put(file.Name(), "target/blob")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.PutCallCount()).To(Equal(1))
			path, _, _ := fakeStorageClient.PutArgsForCall(0)
			Expect(path).To(Equal("target/blob"))
		})

		It("fails if the source file does not exist", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Put("nonexistent/path", "target/blob")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to open source file"))
			Expect(fakeStorageClient.PutCallCount()).To(Equal(0))
		})
	})

	Context("Get", func() {
		It("downloads a blob to a file", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			content := io.NopCloser(strings.NewReader("test content"))
			fakeStorageClient.GetReturns(content, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)

			tmpFile, err := os.CreateTemp("", "download")
			Expect(err).NotTo(HaveOccurred())
			tmpFile.Close()                 //nolint:errcheck
			defer os.Remove(tmpFile.Name()) //nolint:errcheck

			err = davBlobstore.Get("source/blob", tmpFile.Name())

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.GetCallCount()).To(Equal(1))

			downloaded, err := os.ReadFile(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(downloaded)).To(Equal("test content"))
		})
	})

	Context("Delete", func() {
		It("deletes a blob", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.DeleteReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Delete("blob/path")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.DeleteCallCount()).To(Equal(1))
			Expect(fakeStorageClient.DeleteArgsForCall(0)).To(Equal("blob/path"))
		})
	})

	Context("Exists", func() {
		It("returns true if blob exists", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ExistsReturns(true, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			exists, err := davBlobstore.Exists("blob/path")

			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeStorageClient.ExistsCallCount()).To(Equal(1))
		})

		It("returns false if blob does not exist", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ExistsReturns(false, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			exists, err := davBlobstore.Exists("blob/path")

			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("returns error on server error", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ExistsReturns(false, fmt.Errorf("server error"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			exists, err := davBlobstore.Exists("blob/path")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("server error"))
			Expect(exists).To(BeFalse())
		})
	})
})
