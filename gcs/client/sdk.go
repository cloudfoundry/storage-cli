/*
 * Copyright 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/oauth2/google"

	"google.golang.org/api/option"

	"net/http"

	"cloud.google.com/go/storage"
	"github.com/cloudfoundry/storage-cli/gcs/config"
)

const uaString = "storage-cli-gcs"

func newStorageClients(ctx context.Context, cfg *config.GCSCli) (*storage.Client, *storage.Client, error) {
	publicClient, err := storage.NewClient(ctx, option.WithUserAgent(uaString), option.WithHTTPClient(http.DefaultClient))
	var authenticatedClient *storage.Client

	switch cfg.CredentialsSource {
	case config.NoneCredentialsSource:
		// no-op
	case config.DefaultCredentialsSource:
		if tokenSource, err := google.DefaultTokenSource(ctx, storage.ScopeFullControl); err == nil {
			authenticatedClient, err = storage.NewClient(ctx, option.WithUserAgent(uaString), option.WithTokenSource(tokenSource)) //nolint:ineffassign,staticcheck
		}
	case config.ServiceAccountFileCredentialsSource:
		if token, err := google.JWTConfigFromJSON([]byte(cfg.ServiceAccountFile), storage.ScopeFullControl); err == nil {
			authenticatedClient, err = storage.NewClient(ctx, option.WithUserAgent(uaString), option.WithTokenSource(token.TokenSource(ctx))) //nolint:ineffassign,staticcheck
		}
	default:
		return nil, nil, errors.New("unknown credentials_source in configuration")
	}
	return authenticatedClient, publicClient, err
}

// extractProjectID extracts the GCP project ID from credentials
func extractProjectID(ctx context.Context, cfg *config.GCSCli) (string, error) {
	switch cfg.CredentialsSource {
	case config.ServiceAccountFileCredentialsSource:
		// Parse service account JSON to extract project_id
		var serviceAccount struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal([]byte(cfg.ServiceAccountFile), &serviceAccount); err != nil {
			return "", fmt.Errorf("parsing service account JSON: %w", err)
		}
		if serviceAccount.ProjectID == "" {
			return "", errors.New("project_id not found in service account JSON")
		}
		return serviceAccount.ProjectID, nil
		
	case config.DefaultCredentialsSource:
		// Try to get project ID from default credentials
		creds, err := google.FindDefaultCredentials(ctx, storage.ScopeFullControl)
		if err != nil {
			return "", fmt.Errorf("finding default credentials: %w", err)
		}
		if creds.ProjectID == "" {
			return "", errors.New("project_id not found in default credentials")
		}
		return creds.ProjectID, nil
		
	case config.NoneCredentialsSource:
		return "", errors.New("cannot create bucket with read-only credentials")
		
	default:
		return "", errors.New("unknown credentials_source")
	}
}
