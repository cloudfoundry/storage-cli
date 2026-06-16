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
	endpointBase := "https://api.example.com"
	directoryKey := "cc-droplets"

	Context("HMAC Signed URL", func() {
		// URL path: /signed/cc-droplets/fake-object-id
		// nginx $blob_path = cc-droplets/fake-object-id
		// HMAC input: "GET" + "cc-droplets/fake-object-id" + "1566817860" + "900"
		// => "GETcc-droplets/fake-object-id15668178609000"  => base64url(hmac-sha256)
		It("Generates a properly formed URL", func() {
			actual, err := signer.GenerateSignedURL(endpointBase, directoryKey, objectID, verb, timeStamp, duration)
			Expect(err).To(BeNil())
			Expect(actual).To(ContainSubstring("/signed/cc-droplets/fake-object-id"))
			Expect(actual).To(ContainSubstring("ts=1566817860"))
			Expect(actual).To(ContainSubstring("e=900"))
			Expect(actual).To(ContainSubstring("st="))
		})
	})
})
