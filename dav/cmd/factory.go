package cmd

import (
	"crypto/x509"
	"fmt"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshhttpclient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	davclient "github.com/cloudfoundry/storage-cli/dav/client"
	davconf "github.com/cloudfoundry/storage-cli/dav/config"
)

type Factory interface {
	Create(name string) (cmd Cmd, err error)
	SetConfig(config davconf.Config) (err error)
}

func NewFactory(logger boshlog.Logger) Factory {
	return &factory{
		cmds:   make(map[string]Cmd),
		logger: logger,
	}
}

type factory struct {
	config davconf.Config //nolint:unused
	cmds   map[string]Cmd
	logger boshlog.Logger
}

func (f *factory) Create(name string) (cmd Cmd, err error) {
	cmd, found := f.cmds[name]
	if !found {
		err = fmt.Errorf("Could not find command with name %s", name) //nolint:staticcheck
	}
	return
}

func (f *factory) SetConfig(config davconf.Config) (err error) {
	var httpClient boshhttpclient.Client
	var certPool *x509.CertPool

	if len(config.TLS.Cert.CA) != 0 {
		certPool, err = boshcrypto.CertPoolFromPEM([]byte(config.TLS.Cert.CA))
	}

	httpClient = boshhttpclient.CreateDefaultClient(certPool)

	client := davclient.NewClient(config, httpClient, f.logger)

	f.cmds = map[string]Cmd{
		"put":    newPutCmd(client),
		"get":    newGetCmd(client),
		"exists": newExistsCmd(client),
		"delete": newDeleteCmd(client),
		"sign":   newSignCmd(client),
	}

	return
}
