package cmd

import (
	"errors"
	"io"
	"os"

	davclient "github.com/cloudfoundry/storage-cli/dav/client"
)

type GetCmd struct {
	client davclient.Client
}

func newGetCmd(client davclient.Client) (cmd GetCmd) {
	cmd.client = client
	return
}

func (cmd GetCmd) Run(args []string) (err error) {
	if len(args) != 2 {
		err = errors.New("Incorrect usage, get needs remote blob path and local file destination") //nolint:staticcheck
		return
	}

	readCloser, err := cmd.client.Get(args[0])
	if err != nil {
		return
	}
	defer readCloser.Close() //nolint:errcheck

	targetFile, err := os.Create(args[1])
	if err != nil {
		return
	}

	_, err = io.Copy(targetFile, readCloser)
	return
}
