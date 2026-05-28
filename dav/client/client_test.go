package client_test

import (
	"fmt"
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

	Context("Sign", func() {
		var expiry = 100 * time.Second

		It("returns a signed URL for action 'get'", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.SignReturns("https://the-signed-url", nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			url, err := davBlobstore.Sign("blob/path", "get", expiry)

			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://the-signed-url"))

			Expect(fakeStorageClient.SignCallCount()).To(Equal(1))
			object, action, expiration := fakeStorageClient.SignArgsForCall(0)
			Expect(object).To(Equal("blob/path"))
			Expect(action).To(Equal("GET"))
			Expect(expiration).To(Equal(expiry))
		})

		It("returns a signed URL for action 'put'", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.SignReturns("https://the-signed-url", nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			url, err := davBlobstore.Sign("blob/path", "put", expiry)

			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("https://the-signed-url"))

			_, action, _ := fakeStorageClient.SignArgsForCall(0)
			Expect(action).To(Equal("PUT"))
		})

		It("fails on unknown action without calling the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			url, err := davBlobstore.Sign("blob/path", "unknown", expiry)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("action not implemented"))
			Expect(url).To(Equal(""))
			Expect(fakeStorageClient.SignCallCount()).To(Equal(0))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.SignReturns("", fmt.Errorf("boom"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			url, err := davBlobstore.Sign("blob/path", "get", expiry)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("boom"))
			Expect(url).To(Equal(""))
		})
	})

	Context("Copy", func() {
		It("forwards source and destination to the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.CopyReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Copy("src/blob", "dst/blob")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.CopyCallCount()).To(Equal(1))

			src, dst := fakeStorageClient.CopyArgsForCall(0)
			Expect(src).To(Equal("src/blob"))
			Expect(dst).To(Equal("dst/blob"))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.CopyReturns(fmt.Errorf("copy failed"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Copy("src/blob", "dst/blob")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("copy failed"))
		})
	})

	Context("List", func() {
		It("returns the blobs reported by the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ListReturns([]string{"a/b/c", "a/b/d"}, nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			blobs, err := davBlobstore.List("a/b")

			Expect(err).NotTo(HaveOccurred())
			Expect(blobs).To(ConsistOf("a/b/c", "a/b/d"))

			Expect(fakeStorageClient.ListCallCount()).To(Equal(1))
			Expect(fakeStorageClient.ListArgsForCall(0)).To(Equal("a/b"))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.ListReturns(nil, fmt.Errorf("list failed"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			blobs, err := davBlobstore.List("any/prefix")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("list failed"))
			Expect(blobs).To(BeNil())
		})
	})

	Context("DeleteRecursive", func() {
		It("forwards the prefix to the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.DeleteRecursiveReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.DeleteRecursive("some/prefix")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.DeleteRecursiveCallCount()).To(Equal(1))
			Expect(fakeStorageClient.DeleteRecursiveArgsForCall(0)).To(Equal("some/prefix"))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.DeleteRecursiveReturns(fmt.Errorf("recursive delete failed"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.DeleteRecursive("some/prefix")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("recursive delete failed"))
		})
	})

	Context("Properties", func() {
		It("forwards the destination to the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.PropertiesReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Properties("blob/path")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.PropertiesCallCount()).To(Equal(1))
			Expect(fakeStorageClient.PropertiesArgsForCall(0)).To(Equal("blob/path"))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.PropertiesReturns(fmt.Errorf("properties failed"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.Properties("blob/path")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("properties failed"))
		})
	})

	Context("EnsureStorageExists", func() {
		It("delegates to the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.EnsureStorageExistsReturns(nil)

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.EnsureStorageExists()

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStorageClient.EnsureStorageExistsCallCount()).To(Equal(1))
		})

		It("propagates errors from the storage client", func() {
			fakeStorageClient := &clientfakes.FakeStorageClient{}
			fakeStorageClient.EnsureStorageExistsReturns(fmt.Errorf("ensure failed"))

			davBlobstore := client.NewWithStorageClient(fakeStorageClient)
			err := davBlobstore.EnsureStorageExists()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ensure failed"))
		})
	})
})
