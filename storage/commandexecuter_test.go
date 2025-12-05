package storage

import (
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Execute Command", func() {
	var sourceFileName = "some-source-file-command-executer"
	var commandExecuter *CommandExecuter
	var fakeStorager *FakeStorager
	var tempFile *os.File

	BeforeEach(func() {
		fakeStorager = &FakeStorager{}
		commandExecuter = &CommandExecuter{str: fakeStorager}
	})

	Context("Put", func() {
		It("Successfull", func() {
			tempFile, _ = os.CreateTemp("", sourceFileName) //nolint:errcheck
			tempFile.Close()                                //nolint:errcheck
			DeferCleanup(func() {
				os.Remove(tempFile.Name()) //nolint:errcheck
			})
			err := commandExecuter.Execute("put", []string{tempFile.Name(), "destination"})
			Expect(fakeStorager.PutCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("No Source File", func() {
			err := commandExecuter.Execute("put", []string{"source", "destination"})
			Expect(errors.Unwrap(err).Error()).To(ContainSubstring("no such file or directory"))
		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("put", []string{"source"})
			Expect(err.Error()).To(ContainSubstring("put method expected 2 arguments got"))
		})

	})

	Context("Get", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("get", []string{"source", "destination"})
			Expect(fakeStorager.GetCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("get", []string{"source"})
			Expect(err.Error()).To(ContainSubstring("get method expected 2 arguments got"))
		})

	})

	Context("Copy", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("copy", []string{"source", "destination"})
			Expect(fakeStorager.CopyCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("copy", []string{"source"})
			Expect(err.Error()).To(ContainSubstring("copy method expected 2 arguments got"))
		})

	})

	Context("Delete", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("delete", []string{"destination"})
			Expect(fakeStorager.DeleteCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("delete", []string{})
			Expect(err.Error()).To(ContainSubstring("delete method expected 1 argument got"))
		})

	})

	Context("Delete-Recursive", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("delete-recursive", []string{})
			Expect(fakeStorager.DeleteRecursiveCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.deleteRecursiveArgsForCall[0].arg1).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Successfull With Prefix", func() {
			err := commandExecuter.Execute("delete-recursive", []string{"prefix"})
			Expect(fakeStorager.DeleteRecursiveCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.deleteRecursiveArgsForCall[0].arg1).To(Equal("prefix"))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("delete-recursive", []string{"prefix", "extra-prefix"})
			Expect(err.Error()).To(ContainSubstring("delete-recursive takes at most 1 argument (prefix) got"))
		})

	})

	Context("Exists", func() {
		It("Successfull", func() {
			fakeStorager.ExistsStub = func(file string) (bool, error) {
				return true, nil
			}
			err := commandExecuter.Execute("exists", []string{"object"})

			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Not found", func() {
			fakeStorager.ExistsStub = func(file string) (bool, error) {
				return false, nil
			}

			err := commandExecuter.Execute("exists", []string{"object"})
			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).To(BeAssignableToTypeOf(&NotExistsError{}))

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("exists", []string{"object", "extra-object"})
			Expect(err.Error()).To(ContainSubstring("exists method expected 1 argument got"))
		})

	})

	Context("Sign", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("sign", []string{"object", "put", "10s"})

			Expect(fakeStorager.SignCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong action", func() {
			err := commandExecuter.Execute("sign", []string{"object", "delete", "10s"})
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("action not implemented: %s. Available actions are 'get' and 'put'", "delete")))

		})

		It("Wrong time format", func() {
			err := commandExecuter.Execute("sign", []string{"object", "put", "10"})
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", "10")))

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("sign", []string{"object", "put"})
			Expect(err.Error()).To(ContainSubstring("sign method expects 3 arguments got"))

		})

	})

	Context("List", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("list", []string{})

			Expect(fakeStorager.ListCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.listArgsForCall[0].arg1).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())

		})

		It("With Prefix", func() {
			err := commandExecuter.Execute("exists", []string{"prefix"})
			Expect(fakeStorager.ExistsCallCount()).To(BeEquivalentTo(1))
			Expect(fakeStorager.existsArgsForCall[0].arg1).To(Equal("prefix"))
			Expect(err).To(BeAssignableToTypeOf(&NotExistsError{}))

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("list", []string{"prefix-1", "prefix-2"})
			Expect(err.Error()).To(ContainSubstring("list method takes at most 1 argument (prefix) got"))
		})

	})

	Context("Properties", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("properties", []string{"object"})
			Expect(fakeStorager.PropertiesCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("properties", []string{})
			Expect(err.Error()).To(ContainSubstring("properties method expected 1 argument got"))
		})

	})

	Context("Ensure storage exists", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("ensure-storage-exists", []string{})
			Expect(fakeStorager.EnsureStorageExistsCallCount()).To(BeEquivalentTo(1))
			Expect(err).ToNot(HaveOccurred())

		})

		It("Wrong number of parameters", func() {
			err := commandExecuter.Execute("ensure-storage-exists", []string{"extra-parameter"})
			Expect(err.Error()).To(ContainSubstring("ensureStorageExists method expected 0 argument got"))
		})

	})

	Context("Unsupported command", func() {
		It("Successfull", func() {
			err := commandExecuter.Execute("unsupported-command", []string{})
			Expect(err.Error()).To(ContainSubstring("unknown command: '%s'", "unsupported-command"))

		})

	})

})
