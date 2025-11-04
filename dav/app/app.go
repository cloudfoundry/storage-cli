package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	davcmd "github.com/cloudfoundry/storage-cli/dav/cmd"
	davconfig "github.com/cloudfoundry/storage-cli/dav/config"
)

type App struct {
	runner davcmd.Runner
}

func New(runner davcmd.Runner) (app App) {
	app.runner = runner
	return
}

func (app App) Run(args []string) (err error) {
	args = args[1:]
	var configFilePath string
	var printVersion bool

	flagSet := flag.NewFlagSet("davcli-args", flag.ContinueOnError)
	flagSet.StringVar(&configFilePath, "c", "", "Config file path")
	flagSet.BoolVar(&printVersion, "v", false, "print version info")

	err = flagSet.Parse(args)
	if err != nil {
		return
	}

	if printVersion {
		fmt.Println("davcli version [[version]]")
		return
	}

	if configFilePath == "" {
		err = errors.New("Config file arg `-c` is missing") //nolint:staticcheck
		return
	}

	file, err := os.Open(configFilePath)
	if err != nil {
		return
	}

	configBytes, err := io.ReadAll(file)
	if err != nil {
		return
	}

	config := davconfig.Config{}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return
	}

	err = app.runner.SetConfig(config)
	if err != nil {
		err = fmt.Errorf("Invalid CA Certificate: %s", err.Error()) //nolint:staticcheck
		return
	}

	err = app.runner.Run(args[2:])
	return
}
