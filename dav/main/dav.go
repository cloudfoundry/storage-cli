package main

import (
	"fmt"
	"os"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/storage-cli/dav/app"
	"github.com/cloudfoundry/storage-cli/dav/cmd"
)

func main() {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	cmdFactory := cmd.NewFactory(logger)

	cmdRunner := cmd.NewRunner(cmdFactory)

	cli := app.New(cmdRunner)

	err := cli.Run(os.Args)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			fmt.Printf("Blob not found - %s", err.Error())
			os.Exit(3)
		}
		fmt.Printf("Error running app - %s", err.Error())
		os.Exit(1)
	}
}
