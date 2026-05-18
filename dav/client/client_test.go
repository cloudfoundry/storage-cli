package client_test

import (
	"io"
	"os"
	"strings"
	"time"

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

	Context("DeleteRecursive", func() {
		It("lists and deletes all blobs with prefix", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ListReturns([]string{"blob1", "blob2", "blob3"}, nil)
			fakeStorageClient.DeleteReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.DeleteRecursive("prefix/")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.ListCallCount()).To(Equal(1))
			Expect(fakeStorageClient.DeleteCallCount()).To(Equal(3))
		})
	})

	Context("Exists", func() {
		It("returns true when blob exists", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ExistsReturns(true, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			exists, err := davBlobstore.Exists("somefile")

			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
			Expect(fakeStorageClient.ExistsCallCount()).To(Equal(1))
			Expect(fakeStorageClient.ExistsArgsForCall(0)).To(Equal("somefile"))
		})

		It("returns false when blob does not exist", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ExistsReturns(false, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			exists, err := davBlobstore.Exists("somefile")

			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
			Expect(fakeStorageClient.ExistsCallCount()).To(Equal(1))
		})
	})

	Context("List", func() {
		It("returns list of blobs", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ListReturns([]string{"blob1.txt", "blob2.txt"}, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			blobs, err := davBlobstore.List("prefix/")

			Expect(err).NotTo(HaveOccurred())
			Expect(blobs).To(Equal([]string{"blob1.txt", "blob2.txt"}))
			Expect(fakeStorageClient.ListCallCount()).To(Equal(1))
			Expect(fakeStorageClient.ListArgsForCall(0)).To(Equal("prefix/"))
		})
	})

	Context("Copy", func() {
		It("copies a blob from source to destination", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.CopyReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Copy("source/blob", "dest/blob")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.CopyCallCount()).To(Equal(1))
			src, dst := fakeStorageClient.CopyArgsForCall(0)
			Expect(src).To(Equal("source/blob"))
			Expect(dst).To(Equal("dest/blob"))
		})
	})

	Context("Sign", func() {
		It("generates a signed URL", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.SignReturns("https://signed-url.com", nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			signedURL, err := davBlobstore.Sign("blob/path", "get", 1*time.Hour)

			Expect(err).NotTo(HaveOccurred())
			Expect(signedURL).To(Equal("https://signed-url.com"))
			Expect(fakeStorageClient.SignCallCount()).To(Equal(1))
			path, action, duration := fakeStorageClient.SignArgsForCall(0)
			Expect(path).To(Equal("blob/path"))
			Expect(action).To(Equal("get"))
			Expect(duration).To(Equal(1 * time.Hour))
		})
	})

	Context("Properties", func() {
		It("retrieves blob properties", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.PropertiesReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Properties("blob/path")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.PropertiesCallCount()).To(Equal(1))
			Expect(fakeStorageClient.PropertiesArgsForCall(0)).To(Equal("blob/path"))
		})
	})

	Context("EnsureStorageExists", func() {
		It("ensures storage is initialized", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.EnsureStorageExistsReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.EnsureStorageExists()

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.EnsureStorageExistsCallCount()).To(Equal(1))
		})
	})
})
