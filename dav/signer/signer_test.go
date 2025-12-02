package signer_test

import (
	"time"

	"github.com/cloudfoundry/storage-cli/dav/signer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signer", func() {
	secret := "mefq0umpmwevpv034m890j34m0j0-9!fijm434j99j034mjrwjmv9m304mj90;2ef32buf32gbu2i3"
	objectID := "fake-object-id"
	verb := "get"
	signer := signer.NewSigner(secret)
	duration := time.Duration(15 * time.Minute)
	timeStamp := time.Date(2019, 8, 26, 11, 11, 0, 0, time.UTC)
	path := "https://api.example.com/"

	Context("HMAC Signed URL", func() {

		expected := "https://api.example.com/signed/fake-object-id?e=900&st=BxLKZK_dTSLyBis1pAjdwq4aYVrJvXX6vvLpdCClGYo&ts=1566817860"

		It("Generates a properly formed URL", func() {
			actual, err := signer.GenerateSignedURL(path, objectID, verb, timeStamp, duration)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(expected))
		})
	})
})
