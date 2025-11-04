package cmd

import (
	"errors"

	davclient "github.com/cloudfoundry/storage-cli/dav/client"
)

type DeleteCmd struct {
	client davclient.Client
}

func newDeleteCmd(client davclient.Client) (cmd DeleteCmd) {
	cmd.client = client
	return
}

func (cmd DeleteCmd) Run(args []string) (err error) {
	if len(args) != 1 {
		err = errors.New("Incorrect usage, delete needs remote blob path") //nolint:staticcheck
		return
	}
	err = cmd.client.Delete(args[0])
	return
}
