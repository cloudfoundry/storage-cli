package app

import (
	"errors"
	"fmt"
	"time"

	davcmd "github.com/cloudfoundry/storage-cli/dav/cmd"
	davconfig "github.com/cloudfoundry/storage-cli/dav/config"
)

type App struct {
	runner davcmd.Runner
	config davconfig.Config
}

func New(r davcmd.Runner, c davconfig.Config) *App {
	app := &App{runner: r, config: c}
	return app
}

func (app *App) run(args []string) (err error) {

	err = app.runner.SetConfig(app.config)
	if err != nil {
		err = fmt.Errorf("Invalid CA Certificate: %s", err.Error()) //nolint:staticcheck
		return
	}

	err = app.runner.Run(args)
	return
}

func (app *App) Put(sourceFilePath string, destinationObject string) error {
	return app.run([]string{"put", sourceFilePath, destinationObject})
}

func (app *App) Get(sourceObject string, dest string) error {
	return app.run([]string{"get", sourceObject, dest})
}

func (app *App) Delete(object string) error {
	return app.run([]string{"delete", object})
}

func (app *App) Exists(object string) (bool, error) {
	err := app.run([]string{"exists", object})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (app *App) Sign(object string, action string, expiration time.Duration) (string, error) {
	err := app.run([]string{"sign", object, action, expiration.String()})
	if err != nil {
		return "", err
	}
	return "", nil
}

func (app *App) List(prefix string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (app *App) Copy(srcBlob string, dstBlob string) error {
	return errors.New("not implemented")
}

func (app *App) Properties(dest string) error {
	return errors.New("not implemented")
}

func (app *App) EnsureStorageExists() error {
	return errors.New("not implemented")
}

func (app *App) DeleteRecursive(prefix string) error {
	return errors.New("not implemented")
}
