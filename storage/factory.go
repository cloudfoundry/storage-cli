package storage

import (
	"context"
	"fmt"
	"os"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	alioss "github.com/cloudfoundry/storage-cli/alioss/client"
	aliossconfig "github.com/cloudfoundry/storage-cli/alioss/config"
	azurebs "github.com/cloudfoundry/storage-cli/azurebs/client"
	azureconfigbs "github.com/cloudfoundry/storage-cli/azurebs/config"
	davapp "github.com/cloudfoundry/storage-cli/dav/app"
	davcmd "github.com/cloudfoundry/storage-cli/dav/cmd"
	davconfig "github.com/cloudfoundry/storage-cli/dav/config"
	gcs "github.com/cloudfoundry/storage-cli/gcs/client"
	gcsconfig "github.com/cloudfoundry/storage-cli/gcs/config"
	s3 "github.com/cloudfoundry/storage-cli/s3/client"
	s3config "github.com/cloudfoundry/storage-cli/s3/config"
)

var newAzurebsClient = func(configFile *os.File) (Storager, error) {
	conf, err := azureconfigbs.NewFromReader(configFile)
	if err != nil {
		return nil, err
	}

	sc, err := azurebs.NewStorageClient(conf)
	if err != nil {
		return nil, err
	}

	azClient, err := azurebs.New(sc)
	if err != nil {
		return nil, err
	}
	return &azClient, nil
}

var newAliossClient = func(configFile *os.File) (Storager, error) {
	aliConfig, err := aliossconfig.NewFromReader(configFile)
	if err != nil {
		return nil, err
	}

	storageClient, err := alioss.NewStorageClient(aliConfig)
	if err != nil {
		return nil, err
	}

	aliClient, err := alioss.New(storageClient)
	if err != nil {
		return nil, err
	}

	return &aliClient, nil
}

var newGcsClient = func(configFile *os.File) (Storager, error) {
	gcsConfig, err := gcsconfig.NewFromReader(configFile)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	gcsClient, err := gcs.New(ctx, &gcsConfig)
	if err != nil {
		return nil, err
	}
	return gcsClient, nil

}

var newS3Client = func(configFile *os.File) (Storager, error) {
	s3Config, err := s3config.NewFromReader(configFile)
	if err != nil {
		return nil, err
	}

	s3Client, err := s3.NewAwsS3Client(&s3Config)
	if err != nil {
		return nil, err
	}

	return s3.New(s3Client, &s3Config), nil

}

var newDavClient = func(configFile *os.File) (Storager, error) {
	davConfig, err := davconfig.NewFromReader(configFile)
	if err != nil {
		return nil, err
	}

	logger := boshlog.NewLogger(boshlog.LevelNone)
	cmdFactory := davcmd.NewFactory(logger)

	cmdRunner := davcmd.NewRunner(cmdFactory)

	return davapp.New(cmdRunner, davConfig), nil
}

func NewStorageClient(storageType string, configFile *os.File) (Storager, error) {
	switch storageType {
	case "azurebs":
		return newAzurebsClient(configFile)
	case "alioss":
		return newAliossClient(configFile)
	case "s3":
		return newS3Client(configFile)
	case "gcs":
		return newGcsClient(configFile)
	case "dav":
		return newDavClient(configFile)
	default:
		return nil, fmt.Errorf("storage %s not implemented", storageType)
	}
}
