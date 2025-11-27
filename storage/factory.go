package storage

import (
	"context"
	"errors"
	"fmt"
	"os"

	alioss "github.com/cloudfoundry/storage-cli/alioss/client"
	aliossconfig "github.com/cloudfoundry/storage-cli/alioss/config"
	azurebs "github.com/cloudfoundry/storage-cli/azurebs/client"
	azureconfigbs "github.com/cloudfoundry/storage-cli/azurebs/config"
	gcs "github.com/cloudfoundry/storage-cli/gcs/client"
	gcsconfig "github.com/cloudfoundry/storage-cli/gcs/config"
	s3 "github.com/cloudfoundry/storage-cli/s3/client"
	s3config "github.com/cloudfoundry/storage-cli/s3/config"
)

func NewStorageClient(storageType string, configFile *os.File) (Storager, error) {
	var client Storager

	switch storageType {
	case "azurebs":
		{

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
			client = &azClient

		}
	case "alioss":
		{
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

			client = &aliClient
		}
	case "s3":
		{
			s3Config, err := s3config.NewFromReader(configFile)
			if err != nil {
				return nil, err
			}

			s3Client, err := s3.NewAwsS3Client(&s3Config)
			if err != nil {
				return nil, err
			}

			client = s3.New(s3Client, &s3Config)
		}
	case "gcs":
		{
			gcsConfig, err := gcsconfig.NewFromReader(configFile)
			if err != nil {
				return nil, err
			}

			ctx := context.Background()
			gcsClient, err := gcs.New(ctx, &gcsConfig)
			if err != nil {
				return nil, err
			}
			client = gcsClient
		}
	case "dav":
		{
			return nil, errors.New("dav storage provider not implemented yet")
		}

	default:
		return nil, fmt.Errorf("storage %s provider not implemented", storageType)
	}

	return client, nil

}
