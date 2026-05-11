# NEW storage-cli Signing Flow - Detailed Analysis

## Overview

The NEW storage-cli WebDAV client supports **TWO SEPARATE signing methods** through optional commands:
1. `sign-internal` - For Diego cells (internal network)
2. `sign-public` - For external users via CF API

**CRITICAL:** These methods are called **ON DEMAND** (lazy signing) by CCNG when needed, matching the OLD WebDAV client behavior.

---

## Architecture Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG) - Ruby                                              │
│                                                                              │
│  ┌──────────────────────┐    ┌──────────────────────┐                      │
│  │ StorageCliClient     │───>│ StorageCliBlob       │                      │
│  │                      │    │                      │                      │
│  │ - blob(key)          │    │ - @storage_cli_client│                      │
│  │ - sign_internal_url  │    │ - @key               │                      │
│  │ - sign_public_url    │    │ - internal_download_ │                      │
│  │ - supports_lazy_     │    │   url                │                      │
│  │   signing? => true   │    │ - public_download_url│                      │
│  │   (only for DAV)     │    │                      │                      │
│  └──────────────────────┘    └──────────────────────┘                      │
│           │                            │                                     │
│           │ Calls storage-cli          │ Calls on-demand                    │
│           ▼                            ▼                                     │
└───────────────────────────────────────────────────────────────────────────┘
            │
            │ Shell execution: storage-cli -s dav -c config.json sign-internal ...
            │                  storage-cli -s dav -c config.json sign-public ...
            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ storage-cli (Go)                                                             │
│                                                                              │
│  ┌──────────────────────┐    ┌──────────────────────┐    ┌──────────────┐ │
│  │ CommandExecuter      │───>│ DavBlobstore         │───>│ storageClient│ │
│  │                      │    │                      │    │              │ │
│  │ - Execute("sign-    │    │ - SignInternal()     │    │ - SignInternal│ │
│  │    internal")        │    │ - SignPublic()       │    │ - SignPublic │ │
│  │ - Type assertion:    │    │                      │    │ - signVia    │ │
│  │   if SignerInternal  │    │ (implements optional │    │   External   │ │
│  │   supported          │    │  interface)          │    │   Endpoint   │ │
│  └──────────────────────┘    └──────────────────────┘    └──────────────┘ │
│                                                                   │          │
│                                                    Calls /sign    │          │
│                                                    endpoint       ▼          │
└─────────────────────────────────────────────────────────────────────────────┘
                                                                    │
                        HTTP GET to external signer                │
                        (blobstore_url_signer service)             │
                                                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ External Signer (blobstore_url_signer)                                      │
│                                                                              │
│  Generates MD5 signature for path and returns signed URL                    │
│  Returns: http://blobstore.service.cf.internal/read/{path}?md5=...&expires= │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Configuration Setup

### BOSH Manifest (cf-deployment)

```yaml
# Droplets blobstore config
cc:
  droplets:
    droplet_directory_key: cc-droplets
    storage_cli_config_file_droplets: /var/vcap/jobs/cloud_controller_ng/config/droplets.json

# Config file content (droplets.json)
{
  "provider": "dav",
  "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",
  "public_endpoint": "https://blobstore.example.com/admin/cc-droplets",
  "user": "blobstore-user",
  "password": "secret123",
  "signed_url_format": "external-nginx-secure-link-signer",
  "tls": {
    "cert": {
      "ca": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
    }
  }
}
```

### StorageCliClient Initialization

```ruby
# lib/cloud_controller/blobstore/client_provider.rb
StorageCliClient.new(
  directory_key: 'cc-droplets',
  resource_type: 'droplets',
  root_dir: nil,
  min_size: 0,
  max_size: nil
)
```

**StorageCliClient initialization:**

```ruby
def initialize(directory_key:, resource_type:, root_dir:, min_size: nil, max_size: nil)
  config_file_path = config_path_for(resource_type)
  # => /var/vcap/jobs/cloud_controller_ng/config/droplets.json
  
  cfg = fetch_config(resource_type)
  # Reads JSON file, gets provider: "dav"
  
  @storage_type = 'dav'  # Determines lazy signing support
  @cli_path = '/var/vcap/packages/storage-cli/bin/storage-cli'
  @config_file = config_file_path
  @directory_key = directory_key  # cc-droplets
  @resource_type = 'droplets'
  @root_dir = root_dir
end
```

**Key point:** The config file contains BOTH endpoints:
- `endpoint` = `https://blobstore.service.cf.internal:4443/admin/cc-droplets` (internal)
- `public_endpoint` = `https://blobstore.example.com/admin/cc-droplets` (public)

---

## Flow 1: Diego Downloads (Internal)

### Step 1: CCNG Prepares Staging Task

```ruby
# lib/cloud_controller/diego/buildpack/staging_action_builder.rb
# or similar staging/running code

# Get blob for droplet
blob = droplet_blobstore.blob(droplet.guid)
# blob is a StorageCliBlob instance with @storage_cli_client reference

# Generate internal download URL for Diego
download_url = blob.internal_download_url
```

### Step 2: StorageCliClient.blob (Lazy Signing Setup)

```ruby
# lib/cloud_controller/blobstore/storage_cli/storage_cli_client.rb

def blob(key)
  properties = properties(key)
  return nil if properties.nil? || properties.empty?
  
  # For DAV with lazy signing support, pass client reference for on-demand signing
  # For other providers (S3, Azure, GCS), generate signed URL eagerly
  if supports_lazy_signing?
    StorageCliBlob.new(key, properties:, storage_cli_client: self, expires_in_seconds: 3600)
  else
    signed_url = sign_url(partitioned_key(key), verb: 'get', expires_in_seconds: 3600)
    StorageCliBlob.new(key, properties:, signed_url:)
  end
end

def supports_lazy_signing?
  # Only DAV with external signer needs lazy signing for internal vs public endpoints
  @storage_type == 'dav'
end
```

**Key difference from OLD client:**
- OLD: DavClient created DavBlob with NginxSecureLinkSigner reference
- NEW: StorageCliClient creates StorageCliBlob with self reference (only for DAV)

### Step 3: StorageCliBlob.internal_download_url

```ruby
# lib/cloud_controller/blobstore/storage_cli/storage_cli_blob.rb

def internal_download_url
  # For DAV with lazy signing support, generate URL on-demand
  if @storage_cli_client&.supports_lazy_signing?
    return @storage_cli_client.sign_internal_url(@key, verb: 'get', expires_in_seconds: @expires_in_seconds)
  end
  
  # For other providers or DAV without lazy signing, use pre-generated URL
  signed_url
end
```

**Input:**
- `@key` = `"dr/op/droplet-guid"` (already partitioned by CCNG)
- `@expires_in_seconds` = `3600`

### Step 4: StorageCliClient.sign_internal_url

```ruby
# lib/cloud_controller/blobstore/storage_cli/storage_cli_client.rb

def sign_internal_url(key, verb:, expires_in_seconds:)
  stdout, _status = run_cli('sign-internal', partitioned_key(key), verb.to_s.downcase, "#{expires_in_seconds}s")
  stdout.strip
end

private

def run_cli(command, *args, allow_exit_code_three: false)
  # Example command:
  # /var/vcap/packages/storage-cli/bin/storage-cli \
  #   -s dav \
  #   -c /var/vcap/jobs/cloud_controller_ng/config/droplets.json \
  #   sign-internal dr/op/droplet-guid get 3600s
  
  stdout, stderr, status = Open3.capture3(
    @cli_path, '-s', @storage_type, '-c', @config_file, 
    *additional_flags, command, *args
  )
  
  # Returns the signed URL as stdout
  [stdout, status]
end
```

### Step 5: storage-cli CommandExecuter

```go
// storage/commandexecuter.go

func (sty *CommandExecuter) Execute(cmd string, nonFlagArgs []string) error {
  switch cmd {
  case "sign-internal":
    if len(nonFlagArgs) != 3 {
      return fmt.Errorf("sign-internal method expects 3 arguments got %d", len(nonFlagArgs))
    }
    
    objectID, action := nonFlagArgs[0], nonFlagArgs[1]  // "dr/op/droplet-guid", "get"
    action = strings.ToLower(action)
    if action != "get" && action != "put" {
      return fmt.Errorf("action not implemented: %s", action)
    }
    
    expiration, err := time.ParseDuration(nonFlagArgs[2])  // "3600s"
    if err != nil {
      return fmt.Errorf("expiration should be in the format of a duration i.e. 1h, 60m, 3600s. Got: %s", nonFlagArgs[2])
    }
    
    // Check if storage provider supports internal/public signing (type assertion)
    if signer, ok := sty.str.(SignerInternal); ok {
      signedURL, err := signer.SignInternal(objectID, action, expiration)
      if err != nil {
        return fmt.Errorf("failed to sign-internal request: %w", err)
      }
      fmt.Print(signedURL)  // Output to stdout for Ruby to capture
    } else {
      return fmt.Errorf("sign-internal is not supported by this storage provider")
    }
  }
}
```

**Key design:**
- Uses **optional interface** `SignerInternal` via type assertion
- Only DAV implements this interface
- Other providers (S3, Azure, GCS, AliOSS) don't need to implement it

### Step 6: DavBlobstore.SignInternal

```go
// dav/client/client.go

func (d *DavBlobstore) SignInternal(dest string, action string, expiration time.Duration) (string, error) {
  slog.Info("Signing internal URL for WebDAV", "dest", dest, "action", action, "expiration", expiration)
  
  signedURL, err := d.storageClient.SignInternal(dest, action, expiration)
  if err != nil {
    return "", fmt.Errorf("failed to sign internal URL: %w", err)
  }
  
  return signedURL, nil
}
```

### Step 7: storageClient.SignInternal

```go
// dav/client/storage_client.go

func (c *storageClient) SignInternal(blobID, action string, duration time.Duration) (string, error) {
  return c.signWithEndpoint(blobID, action, duration, c.config.Endpoint, "internal")
}

func (c *storageClient) signWithEndpoint(blobID, action string, duration time.Duration, endpoint string, endpointType string) (string, error) {
  if err := validateBlobID(blobID); err != nil {
    return "", err
  }
  
  action = strings.ToUpper(action)
  if action != "GET" && action != "PUT" {
    return "", fmt.Errorf("action not implemented: %s", action)
  }
  
  // Check if external signer is configured
  if c.config.SignedURLFormat == "external-nginx-secure-link-signer" {
    return c.signViaExternalEndpoint(blobID, action, duration, endpoint)
  }
  
  // Internal signer (hmac-sha256 or secure-link-md5)
  // ... (not used with external-nginx-secure-link-signer)
}
```

### Step 8: storageClient.signViaExternalEndpoint

```go
// dav/client/storage_client.go

func (c *storageClient) signViaExternalEndpoint(blobID, action string, duration time.Duration, targetEndpoint string) (string, error) {
  // Step 1: Extract sign endpoint (scheme + host + port) and directory key
  // Always use the internal/private endpoint for calling the /sign service
  signEndpoint := extractSignEndpoint(c.config.Endpoint)
  // Input:  "https://blobstore.service.cf.internal:4443/admin/cc-droplets"
  // Output: "https://blobstore.service.cf.internal:4443"
  
  directoryKey := extractDirectoryKey(c.config.Endpoint)
  // Input:  "https://blobstore.service.cf.internal:4443/admin/cc-droplets"
  // Output: "cc-droplets"
  
  // Step 2: Build path WITHOUT /admin prefix (just directory key + blob ID)
  signPath := "/" + directoryKey + "/" + blobID
  // Input:  blobID = "dr/op/droplet-guid"
  // Output: "/cc-droplets/dr/op/droplet-guid"
  
  // Step 3: Call external signer
  expires := time.Now().Unix() + int64(duration.Seconds())
  signURL := fmt.Sprintf("%s/sign?expires=%d&path=%s", signEndpoint, expires, url.QueryEscape(signPath))
  // Output: "https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=%2Fcc-droplets%2Fdr%2Fop%2Fdroplet-guid"
  
  req, err := http.NewRequest("GET", signURL, nil)
  if err != nil {
    return "", fmt.Errorf("creating sign request: %w", err)
  }
  
  if c.config.User != "" {
    req.SetBasicAuth(c.config.User, c.config.Password)
  }
  
  resp, err := c.httpClient.Do(req)
  if err != nil {
    return "", fmt.Errorf("calling external signer: %w", err)
  }
  defer resp.Body.Close()
  
  if resp.StatusCode != http.StatusOK {
    bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
    return "", fmt.Errorf("external signer failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
  }
  
  signedURLBytes, err := io.ReadAll(resp.Body)
  if err != nil {
    return "", fmt.Errorf("reading signed URL response: %w", err)
  }
  
  signedURLStr := strings.TrimSpace(string(signedURLBytes))
  // Returns: "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=..."
  
  // Step 4: Replace scheme+host with target endpoint (internal or public)
  responseURL, err := url.Parse(signedURLStr)
  if err != nil {
    return "", fmt.Errorf("parsing signed URL response: %w", err)
  }
  
  targetURL, err := url.Parse(targetEndpoint)
  if err != nil {
    return "", fmt.Errorf("parsing target endpoint: %w", err)
  }
  
  // Replace scheme and host from the response with our target endpoint
  responseURL.Scheme = targetURL.Scheme  // https
  responseURL.Host = targetURL.Host      // blobstore.service.cf.internal:4443
  
  // Final: "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=..."
  return responseURL.String(), nil
}
```

**Key implementation details:**
- Extracts directory key (`cc-droplets`) from endpoint path (strips `/admin/`)
- Builds sign path as `/{directoryKey}/{blobID}` WITHOUT `/admin` prefix
- Calls external signer at `/sign` endpoint with Basic Auth
- Replaces host in response with **target endpoint** (internal for SignInternal)

### Step 9: External Signer Service (Same as OLD)

```
HTTP GET https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=%2Fcc-droplets%2Fdr%2Fop%2Fdroplet-guid
Authorization: Basic base64(blobstore-user:secret123)
```

**Nginx routes to blobstore_url_signer service:**

```go
// blobstore_url_signer/signer/sign.go

func Sign(expire, path string) string {
    // path = "/cc-droplets/dr/op/droplet-guid"
    // expire = "1778170942"
    
    signature := md5(fmt.Sprintf("%s/read%s %s", expire, path, secret))
    // Input: "1778170942/read/cc-droplets/dr/op/droplet-guid SECRET"
    // Output: MD5 hash, base64-encoded, sanitized (/ → _, + → -, remove =)
    
    return fmt.Sprintf(
        "http://blobstore.service.cf.internal/read%s?md5=%s&expires=%s",
        path, signature, expire
    )
    // Returns: "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"
}
```

**Response:**
```
200 OK
Content-Type: text/plain

http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942
```

### Step 10: storage-cli Replaces Host

```go
// Received from signer
responseURL = "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"

// Parse response
responseURL.Path = "/read/cc-droplets/dr/op/droplet-guid"
responseURL.Query = "md5=XYZ123&expires=1778170942"

// Parse target endpoint (internal)
targetURL = "https://blobstore.service.cf.internal:4443/admin/cc-droplets"
targetURL.Scheme = "https"
targetURL.Host = "blobstore.service.cf.internal:4443"

// Replace scheme and host
responseURL.Scheme = "https"  // from targetURL
responseURL.Host = "blobstore.service.cf.internal:4443"  // from targetURL

// Final result
signedURL = "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"
```

### Step 11: Return to CCNG

```ruby
# StorageCliClient.sign_internal_url returns stdout from storage-cli
stdout = "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"
stdout.strip
# => "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"

# StorageCliBlob.internal_download_url returns this URL
blob.internal_download_url
# => "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"

# CCNG passes this URL to Diego
```

### Step 12: Diego Uses Signed URL

```
Diego Cell downloads droplet:
GET https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942

✓ TLS verification succeeds (Diego has blobstore_tls.ca cert)
✓ No Basic Auth needed (signed URL)
✓ Nginx validates MD5 signature
✓ Returns file content
```

---

## Flow 2: External User Downloads (Public)

### Step 1: User Requests Package Download via CF API

```
GET /v3/packages/:guid/download HTTP/1.1
Authorization: Bearer <CF_TOKEN>
```

### Step 2: PackagesController

```ruby
# app/controllers/v3/packages_controller.rb

def download
  package = PackageModel.find(guid: params[:guid])
  BlobDispatcher.new(
    blobstore: packages_blobstore,
    controller: self
  ).send_or_redirect(guid: package.guid)
end
```

### Step 3: BlobDispatcher

```ruby
# app/controllers/runtime/helpers/blob_dispatcher.rb

def send_or_redirect(guid:)
  blob = @blobstore.blob(guid)
  # blob is a StorageCliBlob instance
  
  if @blobstore.local?
    blob_sender.send_blob(blob, @controller)
  else
    @controller.redirect blob.public_download_url  # ← CALLS public_download_url!
  end
end
```

### Step 4: StorageCliBlob.public_download_url

```ruby
# lib/cloud_controller/blobstore/storage_cli/storage_cli_blob.rb

def public_download_url
  # For DAV with lazy signing support, generate URL on-demand
  if @storage_cli_client&.supports_lazy_signing?
    return @storage_cli_client.sign_public_url(@key, verb: 'get', expires_in_seconds: @expires_in_seconds)
  end
  
  # For other providers or DAV without lazy signing, use pre-generated URL
  signed_url
end
```

**Input:**
- `@key` = `"pa/ck/package-guid"` (already partitioned by CCNG)
- `@expires_in_seconds` = `3600`

### Step 5: StorageCliClient.sign_public_url

```ruby
# lib/cloud_controller/blobstore/storage_cli/storage_cli_client.rb

def sign_public_url(key, verb:, expires_in_seconds:)
  stdout, _status = run_cli('sign-public', partitioned_key(key), verb.to_s.downcase, "#{expires_in_seconds}s")
  stdout.strip
end
```

**Shell command:**
```bash
/var/vcap/packages/storage-cli/bin/storage-cli \
  -s dav \
  -c /var/vcap/jobs/cloud_controller_ng/config/packages.json \
  sign-public pa/ck/package-guid get 3600s
```

### Step 6: storage-cli CommandExecuter

```go
// storage/commandexecuter.go

case "sign-public":
  if len(nonFlagArgs) != 3 {
    return fmt.Errorf("sign-public method expects 3 arguments got %d", len(nonFlagArgs))
  }
  
  objectID, action := nonFlagArgs[0], nonFlagArgs[1]  // "pa/ck/package-guid", "get"
  action = strings.ToLower(action)
  
  expiration, err := time.ParseDuration(nonFlagArgs[2])  // "3600s"
  if err != nil {
    return fmt.Errorf("expiration should be in the format of a duration")
  }
  
  // Check if storage provider supports internal/public signing
  if signer, ok := sty.str.(SignerInternal); ok {
    signedURL, err := signer.SignPublic(objectID, action, expiration)
    if err != nil {
      return fmt.Errorf("failed to sign-public request: %w", err)
    }
    fmt.Print(signedURL)  // Output to stdout
  } else {
    return fmt.Errorf("sign-public is not supported by this storage provider")
  }
```

### Step 7: DavBlobstore.SignPublic

```go
// dav/client/client.go

func (d *DavBlobstore) SignPublic(dest string, action string, expiration time.Duration) (string, error) {
  slog.Info("Signing public URL for WebDAV", "dest", dest, "action", action, "expiration", expiration)
  
  signedURL, err := d.storageClient.SignPublic(dest, action, expiration)
  if err != nil {
    return "", fmt.Errorf("failed to sign public URL: %w", err)
  }
  
  return signedURL, nil
}
```

### Step 8: storageClient.SignPublic

```go
// dav/client/storage_client.go

func (c *storageClient) SignPublic(blobID, action string, duration time.Duration) (string, error) {
  // Use public endpoint if configured, otherwise fall back to internal
  endpoint := c.config.PublicEndpoint
  if endpoint == "" {
    endpoint = c.config.Endpoint
  }
  return c.signWithEndpoint(blobID, action, duration, endpoint, "public")
}
```

**Key difference from SignInternal:**
- Uses `c.config.PublicEndpoint` instead of `c.config.Endpoint`
- If `PublicEndpoint` is not configured, falls back to `Endpoint`

### Step 9: storageClient.signViaExternalEndpoint (Public)

```go
func (c *storageClient) signViaExternalEndpoint(blobID, action string, duration time.Duration, targetEndpoint string) (string, error) {
  // Step 1: Extract sign endpoint and directory key
  // ALWAYS use internal endpoint for calling /sign service
  signEndpoint := extractSignEndpoint(c.config.Endpoint)
  // Output: "https://blobstore.service.cf.internal:4443"
  
  directoryKey := extractDirectoryKey(c.config.Endpoint)
  // Output: "cc-packages"
  
  // Step 2: Build path
  signPath := "/" + directoryKey + "/" + blobID
  // Output: "/cc-packages/pa/ck/package-guid"
  
  // Step 3: Call external signer (SAME as internal)
  expires := time.Now().Unix() + int64(duration.Seconds())
  signURL := fmt.Sprintf("%s/sign?expires=%d&path=%s", signEndpoint, expires, url.QueryEscape(signPath))
  // Output: "https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=%2Fcc-packages%2Fpa%2Fck%2Fpackage-guid"
  
  // ... (HTTP request with Basic Auth)
  
  signedURLStr := strings.TrimSpace(string(signedURLBytes))
  // Returns: "http://blobstore.service.cf.internal/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"
  
  // Step 4: Replace host with PUBLIC endpoint (THIS IS THE KEY DIFFERENCE!)
  responseURL, err := url.Parse(signedURLStr)
  targetURL, err := url.Parse(targetEndpoint)
  // targetEndpoint = "https://blobstore.example.com/admin/cc-packages" (public)
  
  // Replace scheme and host
  responseURL.Scheme = targetURL.Scheme  // https
  responseURL.Host = targetURL.Host      // blobstore.example.com
  
  // Final: "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"
  return responseURL.String(), nil
}
```

**Key differences from internal signing:**
- Uses `c.config.PublicEndpoint` as `targetEndpoint`
- Calls SAME external signer service at SAME internal endpoint
- Receives SAME response format
- Only differs in host replacement step

### Step 10: Return to CCNG

```ruby
# StorageCliClient.sign_public_url returns stdout from storage-cli
stdout = "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"
stdout.strip
# => "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"

# StorageCliBlob.public_download_url returns this URL
blob.public_download_url
# => "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"

# BlobDispatcher redirects to this URL
```

### Step 11: CF API Redirects User

```
HTTP/1.1 302 Found
Location: https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942
```

### Step 12: User's Browser Downloads

```
User's browser follows redirect:
GET https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942

✓ TLS verification succeeds (public CA cert)
✓ No Basic Auth needed (signed URL)
✓ Nginx validates MD5 signature
✓ Returns file content
```

---

## Key Comparisons: OLD vs NEW

### Similarities (What Stayed the Same)

1. **Lazy Signing (On-Demand)**
   - OLD: DavBlob calls `@signer.sign_internal_url()` or `@signer.sign_public_url()` when needed
   - NEW: StorageCliBlob calls `@storage_cli_client.sign_internal_url()` or `@storage_cli_client.sign_public_url()` when needed
   - **Both generate signed URLs on-demand, NOT pre-generated**

2. **Same External Signer Service**
   - OLD: NginxSecureLinkSigner calls `/sign` endpoint
   - NEW: storage-cli calls `/sign` endpoint
   - **Same blobstore_url_signer service, same MD5 signature algorithm**

3. **Same Signed URL Format**
   - OLD: `/read/{directoryKey}/{blobID}?md5=...&expires=...`
   - NEW: `/read/{directoryKey}/{blobID}?md5=...&expires=...`
   - **Identical format, same MD5 signature**

4. **Same Endpoint Replacement Logic**
   - OLD: NginxSecureLinkSigner replaces host with `@internal_uri` or `@public_uri`
   - NEW: storage-cli replaces host with `config.Endpoint` or `config.PublicEndpoint`
   - **Same concept: call signer at internal endpoint, replace host for final URL**

5. **Same Two Endpoints**
   - OLD: `private_endpoint` (internal) and `public_endpoint` (public)
   - NEW: `endpoint` (internal) and `public_endpoint` (public)
   - **Both use dual endpoints for internal network vs public internet**

### Differences (What Changed)

| Aspect | OLD WebDAV Client | NEW storage-cli |
|--------|------------------|-----------------|
| **Language** | Pure Ruby (in CCNG process) | Ruby calls Go binary |
| **Signer Component** | NginxSecureLinkSigner (Ruby class) | storage-cli (Go binary) |
| **Process** | In-process (CCNG Ruby) | External process (shell exec) |
| **Interface** | Direct method calls | CLI commands via `Open3.capture3` |
| **Configuration** | Ruby hash in code | JSON config file |
| **Blob Class** | DavBlob | StorageCliBlob |
| **Client Class** | DavClient | StorageCliClient |
| **Lazy Signing Detection** | Always lazy for WebDAV | Only lazy if `supports_lazy_signing?` returns true |
| **Path Handling** | Client appends `/admin/{directoryKey}` to endpoint | Endpoint includes `/admin/{directoryKey}` in config |
| **Two Signing Methods** | `sign_internal_url(path:, expires:)` on signer | `sign-internal` and `sign-public` CLI commands |
| **Optional Interface** | Not applicable (Ruby duck typing) | `SignerInternal` optional interface (Go) |
| **Other Providers** | Separate clients (FogClient, etc.) | Unified storage-cli with `-s` flag |

### Configuration Comparison

**OLD WebDAV Config:**
```yaml
cc:
  droplets:
    fog_connection:
      provider: webdav
      private_endpoint: https://blobstore.service.cf.internal:4443
      public_endpoint: https://blobstore.example.com
      username: blobstore-user
      password: secret123
```

**NEW storage-cli Config:**
```json
{
  "provider": "dav",
  "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",
  "public_endpoint": "https://blobstore.example.com/admin/cc-droplets",
  "user": "blobstore-user",
  "password": "secret123",
  "signed_url_format": "external-nginx-secure-link-signer"
}
```

**Key differences:**
- OLD: `private_endpoint` does NOT include `/admin/{directoryKey}`
- NEW: `endpoint` DOES include `/admin/{directoryKey}`
- OLD: Ruby code appends `/admin/{directoryKey}/{partitioned_key}` when building URLs
- NEW: storage-cli extracts `directoryKey` from endpoint path and builds URLs

### Code Flow Comparison

**OLD: CCNG → DavBlob → NginxSecureLinkSigner → External Signer**
```
CCNG (Ruby)
  ↓
blob.internal_download_url
  ↓
@signer.sign_internal_url(path: "dr/op/droplet-guid", expires: 1778170942)
  ↓
NginxSecureLinkSigner (Ruby)
  - Builds request URI: https://blobstore.service.cf.internal:4443/sign?expires=...&path=/cc-droplets/dr/op/droplet-guid
  - Calls external signer with HTTPClient.get
  - Receives: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...
  - Replaces host with @internal_uri
  - Returns: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...
```

**NEW: CCNG → StorageCliBlob → StorageCliClient → storage-cli → External Signer**
```
CCNG (Ruby)
  ↓
blob.internal_download_url
  ↓
@storage_cli_client.sign_internal_url(@key, verb: 'get', expires_in_seconds: 3600)
  ↓
StorageCliClient (Ruby)
  - Runs: storage-cli -s dav -c config.json sign-internal dr/op/droplet-guid get 3600s
  - Captures stdout
  ↓
storage-cli (Go)
  - CommandExecuter checks if provider implements SignerInternal interface
  - Calls DavBlobstore.SignInternal()
  - Calls storageClient.SignInternal()
  - Calls storageClient.signViaExternalEndpoint()
  - Extracts directoryKey from config.Endpoint
  - Builds request URL: https://blobstore.service.cf.internal:4443/sign?expires=...&path=/cc-droplets/dr/op/droplet-guid
  - Calls external signer with http.Client.Do
  - Receives: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...
  - Replaces host with config.Endpoint (internal)
  - Prints to stdout: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...
  ↓
StorageCliClient (Ruby)
  - Returns stdout.strip
```

---

## Summary

**The NEW storage-cli WebDAV client maintains the SAME lazy signing behavior and dual-endpoint logic as the OLD DavClient**, with these key implementation changes:

1. **Process Boundary**: Ruby calls external Go binary instead of in-process Ruby code
2. **CLI Interface**: Uses shell commands (`sign-internal`, `sign-public`) instead of method calls
3. **Optional Interface**: Uses Go type assertion for WebDAV-specific features
4. **No Impact on Other Providers**: S3, Azure, GCS, AliOSS unchanged

**The signing flow and URL format remain identical:**
- Same external signer service (blobstore_url_signer)
- Same MD5 signature algorithm
- Same `/read/{path}?md5=...&expires=...` URL format
- Same lazy signing (on-demand when `internal_download_url` or `public_download_url` is called)
- Same dual endpoints for internal network vs public internet

**From the perspective of Diego cells and external users, nothing changes:**
- Diego receives: `https://blobstore.service.cf.internal:4443/read/...?md5=...`
- External users receive: `https://blobstore.example.com/read/...?md5=...`
- Both URLs work the same way as with the OLD client
