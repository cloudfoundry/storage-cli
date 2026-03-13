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
	duration := time.Duration(15 * time.Minute)
	timeStamp := time.Date(2019, 8, 26, 11, 11, 0, 0, time.UTC)
	path := "https://api.example.com/"

	Context("SHA256 HMAC Signed URL (BOSH format - default)", func() {
		signer := signer.NewSigner(secret)

		// Expected signature for: HMAC-SHA256("GETfake-object-id1566817860900", secret)
		// timestamp: 1566817860 (2019-08-26 11:11:00 UTC)
		// duration: 900 seconds (15 minutes)
		// Signature matches BOSH secure_link_hmac format: $request_method$object_id$arg_ts$arg_e
		// where arg_e is the DURATION in seconds, not absolute expiration
		expected := "https://api.example.com/signed/fake-object-id?e=900&st=BxLKZK_dTSLyBis1pAjdwq4aYVrJvXX6vvLpdCClGYo&ts=1566817860"

		It("Generates a properly formed URL", func() {
			actual, err := signer.GenerateSignedURL(path, objectID, verb, timeStamp, duration)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(expected))
		})
	})

	Context("SHA256 HMAC Signed URL (BOSH format - explicit)", func() {
		signer, err := signer.NewSignerWithFormat(secret, "sha256")
		Expect(err).To(BeNil())

		expected := "https://api.example.com/signed/fake-object-id?e=900&st=BxLKZK_dTSLyBis1pAjdwq4aYVrJvXX6vvLpdCClGYo&ts=1566817860"

		It("Generates a properly formed URL", func() {
			actual, err := signer.GenerateSignedURL(path, objectID, verb, timeStamp, duration)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(expected))
		})
	})

	Context("MD5 Signed URL (CAPI format)", func() {
		signer, err := signer.NewSignerWithFormat(secret, "md5")
		Expect(err).To(BeNil())

		// Expected signature for: md5("1566818760/read/fake-object-id {secret}")
		// expires: 1566818760 (timestamp + 900 seconds)
		expected := "https://api.example.com/read/fake-object-id?expires=1566818760&md5=WQ0QdFWpV_nxXrlyHPOu6g"

		It("Generates a properly formed URL", func() {
			actual, err := signer.GenerateSignedURL(path, objectID, verb, timeStamp, duration)
			Expect(err).To(BeNil())
			Expect(actual).To(Equal(expected))
		})
	})

	Context("Unsupported format", func() {
		It("Returns an error for unknown format", func() {
			_, err := signer.NewSignerWithFormat(secret, "banana")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("unsupported signed_url_format"))
		})
	})
})
