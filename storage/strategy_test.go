package storage

import (
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Execute Command", func() {
	var sourceFileName = "some-source-file-strategy"
	var strategy *Strategy
	var fakeStorager *FakeStorager
	var tempFile *os.File

	BeforeEach(func() {
		fakeStorager = &FakeStorager{}
		strategy = &Strategy{str: fakeStorager}
	})

	Context("Put", func() {
		// BeforeEach(func() {
		// 	tempFile, _ = os.CreateTemp("", sourceFileName) //nolint:errcheck
		// 	tempFile.Close()                                //nolint:errcheck
		// })

		// AfterEach(func() {
		// 	os.Remove(tempFile.Name()) //nolint:errcheck
		// })

		It("Successfull", func() {
			tempFile, _ = os.CreateTemp("", sourceFileName) //nolint:errcheck
			tempFile.Close()                                //nolint:errcheck
			DeferCleanup(func() {
				os.Remove(tempFile.Name()) //nolint:errcheck
			})
			err := strategy.ExecuteCommand("put", []string{"put", tempFile.Name(), "destination"})
			Expect(fakeStorager.PutCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("No Source File", func() {
			err := strategy.ExecuteCommand("put", []string{"put", "source", "destination"})
			Expect(errors.Unwrap(err).Error()).To(ContainSubstring("no such file or directory"))
		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("put", []string{"put", "source"})
			Expect(err.Error()).To(ContainSubstring("Put method expected 3 arguments got"))
		})

	})

	Context("Get", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("get", []string{"get", "source", "destination"})
			Expect(fakeStorager.GetCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("get", []string{"get", "source"})
			Expect(err.Error()).To(ContainSubstring("Get method expected 3 arguments got"))
		})

	})

	Context("Copy", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("copy", []string{"copy", "source", "destination"})
			Expect(fakeStorager.CopyCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("copy", []string{"copy", "source"})
			Expect(err.Error()).To(ContainSubstring("Copy method expected 3 arguments got"))
		})

	})

	Context("Delete", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("delete", []string{"delete", "destination"})
			Expect(fakeStorager.DeleteCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("delete", []string{"delete"})
			Expect(err.Error()).To(ContainSubstring("Delete method expected 2 arguments got"))
		})

	})

	Context("Delete-Recursive", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("delete-recursive", []string{"delete"})
			Expect(fakeStorager.DeleteRecursiveCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Successfull With Prefix", func() {
			err := strategy.ExecuteCommand("delete-recursive", []string{"delete", "prefix"})
			Expect(fakeStorager.DeleteRecursiveCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.deleteRecursiveArgsForCall[0].arg1).To(Equal("prefix"))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("delete-recursive", []string{"delete", "prefix", "extra-prefix"})
			Expect(err.Error()).To(ContainSubstring("delete-recursive takes at most one argument (prefix) got"))
		})

	})

	Context("Exists", func() {
		It("Successfull", func() {
			fakeStorager.ExistsStub = func(file string) (bool, error) {
				return true, nil
			}
			err := strategy.ExecuteCommand("exists", []string{"exists", "file"})

			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Not found", func() {
			fakeStorager.ExistsStub = func(file string) (bool, error) {
				return false, nil
			}

			err := strategy.ExecuteCommand("exists", []string{"exists", "file"})
			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).To(BeAssignableToTypeOf(&NotExistsError{}))

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("delete-recursive", []string{"delete", "prefix", "extra-prefix"})
			Expect(err.Error()).To(ContainSubstring("delete-recursive takes at most one argument (prefix) got"))
		})

	})

	Context("Sign", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("sign", []string{"sign", "object", "put", "10s"})

			Expect(fakeStorager.SignCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong action", func() {
			err := strategy.ExecuteCommand("sign", []string{"sign", "object", "delete", "10s"})
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Action not implemented: %s. Available actions are 'get' and 'put'", "delete")))

		})

		It("Wrong time format", func() {
			err := strategy.ExecuteCommand("sign", []string{"sign", "object", "put", "10"})
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", "10")))

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("sign", []string{"sign", "object", "put"})
			Expect(err.Error()).To(ContainSubstring("Sign method expects 3 arguments got"))

		})

	})

	Context("List", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("list", []string{"list"})

			Expect(fakeStorager.ListCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("With Prefix", func() {
			err := strategy.ExecuteCommand("exists", []string{"exists", "prefix"})
			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.existsArgsForCall[0].arg1).To(Equal("prefix"))
			Expect(err).To(BeAssignableToTypeOf(&NotExistsError{}))

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("list", []string{})
			Expect(err.Error()).To(ContainSubstring("List method expected 1 or 2 arguments, got"))
		})

	})

	Context("Properties", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("properties", []string{"properties", "object"})
			Expect(fakeStorager.PropertiesCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("properties", []string{"properties"})
			Expect(err.Error()).To(ContainSubstring("Properties method expected 2 arguments got"))
		})

	})

	Context("Ensure storage exists", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("ensure-storage-exists", []string{"ensure-storage-exists"})
			Expect(fakeStorager.EnsureStorageExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := strategy.ExecuteCommand("ensure-storage-exists", []string{"ensure-storage-exists", "extra-parameter"})
			Expect(err.Error()).To(ContainSubstring("EnsureStorageExists method expected 1 arguments got"))
		})

	})

	Context("Unsupported command", func() {
		It("Successfull", func() {
			err := strategy.ExecuteCommand("unsupported-command", []string{"unsupported-command"})
			Expect(err.Error()).To(ContainSubstring("unknown command: '%s'\n", "unsupported-command"))

		})

	})

})
