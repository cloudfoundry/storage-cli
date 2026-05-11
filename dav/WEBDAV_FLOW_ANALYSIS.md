# WebDAV Blobstore Flow Analysis: Old DAV Client vs Storage-CLI (CURRENT)

## Architecture Overview

### Components

1. **Cloud Controller (CCNG)** - Ruby application managing CF resources
2. **Blobstore Nginx** - WebDAV server with two endpoints:
   - **Internal (port 4443)**: `blobstore.service.cf.internal:4443` - TLS with internal CA cert
   - **Public (port 443)**: `blobstore.<system-domain>` - TLS with public CA cert
3. **Blobstore URL Signer** - Go service generating signed URLs
4. **Diego Cell** - Downloads droplets/buildpacks using signed URLs

### Nginx Configuration

```nginx
# Internal Server (blobstore.service.cf.internal:4443)
server {
  listen 4443 ssl;
  root /var/vcap/store/shared/;
  
  location /admin/ {
    # Direct WebDAV operations (PUT, DELETE, COPY, PROPFIND)
    # Requires Basic Auth
    auth_basic "Blobstore Admin";
    auth_basic_user_file write_users;
    dav_methods DELETE PUT COPY;
    dav_ext_methods PROPFIND OPTIONS;
    alias /var/vcap/store/shared/;
  }
  
  location /sign {
    # Calls blobstore_url_signer service
    # Requires Basic Auth
    auth_basic "Blobstore Signing";
    auth_basic_user_file write_users;
    proxy_pass http://blob_url_signer;  # Unix socket
  }
  
  location /read/ {
    # Signed URL downloads (no auth needed)
    secure_link $arg_md5,$arg_expires;
    secure_link_md5 "$secure_link_expires$uri SECRET";
    alias /var/vcap/store/shared/;
  }
}

# Public Server (blobstore.<system-domain>:443)
server {
  listen 443 ssl;
  root /var/vcap/store/shared/;
  
  location /read/ {
    # Public signed URL downloads
    secure_link $arg_md5,$arg_expires;
    secure_link_md5 "$secure_link_expires$uri SECRET";
    alias /var/vcap/store/shared/;
  }
}
```

**Key Points:**
- Both servers route to SAME nginx instance
- Both use SAME `secure_link_md5` secret
- Both serve files from SAME `/var/vcap/store/shared/` directory
- Only difference: hostname and TLS certificate

---

## Operation 1: PUT (Upload Droplet)

### OLD: Ruby DAV Client

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│ lib/cloud_controller/blobstore/webdav/dav_client.rb            │
│                                                                 │
│ cp_to_blobstore("/tmp/droplet.tgz", "droplet-guid")           │
│   │                                                             │
│   ├─ Partition key: "droplet-guid" → "dr/op/droplet-guid"     │
│   │  (via BaseClient.partitioned_key using SHA1)               │
│   │                                                             │
│   └─ url(key) builds:                                          │
│      @endpoint + "/admin/" + @directory_key + "/" + key        │
│      = "https://blobstore.service.cf.internal:4443"           │
│        + "/admin/cc-droplets/dr/op/droplet-guid"               │
│                                                                 │
│   HTTP PUT with Basic Auth                                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ PUT /admin/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            │ Content-Type: application/octet-stream
                            │ Body: <droplet binary>
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal - Port 4443)                         │
│                                                                 │
│ Receives: PUT /admin/cc-droplets/dr/op/droplet-guid           │
│ Matches: location /admin/                                      │
│ Auth: Checks write_users (Basic Auth)                          │
│ Action: dav_methods PUT                                         │
│ Stores: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
│ Response: 201 Created or 204 No Content                        │
└─────────────────────────────────────────────────────────────────┘
```

### NEW: Storage-CLI (CURRENT)

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│ lib/cloud_controller/blobstore/storage_cli/storage_cli_client.rb│
│                                                                 │
│ cp_to_blobstore("/tmp/droplet.tgz", "droplet-guid")           │
│   │                                                             │
│   ├─ Partition key: "droplet-guid" → "dr/op/droplet-guid"     │
│   │  (via BaseClient.partitioned_key using SHA1)               │
│   │                                                             │
│   └─ Execute: storage-cli -s dav -c config.json put \         │
│                /tmp/droplet.tgz dr/op/droplet-guid             │
│                                                                 │
│   Config JSON (/var/vcap/jobs/cloud_controller_ng/config/droplets.json):│
│   {                                                             │
│     "provider": "dav",                                          │
│     "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",│
│     "user": "blobstore-user",                                   │
│     "password": "secret123"                                     │
│   }                                                             │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Spawns subprocess: storage-cli put
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storage-cli Binary (Go)                                         │
│ dav/client/storage_client.go                                    │
│                                                                 │
│ Put("dr/op/droplet-guid", fileReader, fileSize)                │
│   │                                                             │
│   └─ buildBlobURL(blobID):                                     │
│      endpoint + "/" + blobID                                    │
│      = "https://blobstore.service.cf.internal:4443/admin/cc-droplets"│
│        + "/dr/op/droplet-guid"                                  │
│      = "https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid"│
│                                                                 │
│   HTTP PUT with Basic Auth                                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ PUT /admin/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            │ Body: <droplet binary>
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal - Port 4443)                         │
│                                                                 │
│ [IDENTICAL TO OLD CLIENT]                                       │
│ Stores: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────┘
```

**Key Difference:** 
- Old: Ruby code makes HTTP request directly
- New: CLI binary spawned as subprocess, makes HTTP request
- **Result:** IDENTICAL URLs, IDENTICAL behavior

---

## Operation 2: GET (Download via Basic Auth)

### OLD: Ruby DAV Client

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│                                                                 │
│ download_from_blobstore("droplet-guid", "/tmp/droplet.tgz")   │
│   │                                                             │
│   └─ URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid│
│      HTTP GET with Basic Auth                                   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ GET /admin/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                 │
│ Returns: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid│
└─────────────────────────────────────────────────────────────────┘
```

### NEW: Storage-CLI (CURRENT)

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│                                                                 │
│ download_from_blobstore("droplet-guid", "/tmp/droplet.tgz")   │
│   │                                                             │
│   └─ Execute: storage-cli -s dav -c config.json get \         │
│                dr/op/droplet-guid /tmp/droplet.tgz             │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storage-cli Binary                                              │
│                                                                 │
│ Get("dr/op/droplet-guid")                                      │
│   └─ URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid│
│      HTTP GET with Basic Auth                                   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ GET /admin/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                 │
│ [IDENTICAL TO OLD CLIENT]                                       │
└─────────────────────────────────────────────────────────────────┘
```

**Result:** IDENTICAL URLs, IDENTICAL behavior

---

## Operation 3: COPY (Server-Side Copy)

### OLD: Ruby DAV Client

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│                                                                 │
│ cp_file_between_keys("source-guid", "dest-guid")              │
│   │                                                             │
│   ├─ Source: https://blobstore.service.cf.internal:4443/admin/cc-droplets/so/ur/source-guid│
│   ├─ Dest:   https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid│
│   │                                                             │
│   ├─ Step 1: PUT dest (create empty file)                      │
│   │   PUT /admin/cc-droplets/de/st/dest-guid                   │
│   │   Body: empty                                               │
│   │                                                             │
│   └─ Step 2: COPY source → dest                                │
│      COPY /admin/cc-droplets/so/ur/source-guid                 │
│      Destination: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid│
│      Authorization: Basic base64(user:pass)                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                 │
│ Performs server-side copy (no download/upload)                  │
│ Copies: /var/vcap/store/shared/cc-droplets/so/ur/source-guid  │
│     To: /var/vcap/store/shared/cc-droplets/de/st/dest-guid    │
└─────────────────────────────────────────────────────────────────┘
```

### NEW: Storage-CLI (CURRENT)

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│                                                                 │
│ cp_file_between_keys("source-guid", "dest-guid")              │
│   │                                                             │
│   └─ Execute: storage-cli -s dav -c config.json copy \        │
│                so/ur/source-guid de/st/dest-guid               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storage-cli Binary                                              │
│                                                                 │
│ Copy("so/ur/source-guid", "de/st/dest-guid")                   │
│   └─ copyNative()                                              │
│      COPY /admin/cc-droplets/so/ur/source-guid                 │
│      Destination: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid│
│      Overwrite: T                                               │
│      Authorization: Basic base64(user:pass)                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ COPY /admin/cc-droplets/so/ur/source-guid
                            │ Destination: .../de/st/dest-guid
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                 │
│ [IDENTICAL TO OLD CLIENT]                                       │
└─────────────────────────────────────────────────────────────────┘
```

**Key Difference:**
- Old: Two-step (PUT empty + COPY)
- New: Single-step COPY with Overwrite: T header
- **Result:** IDENTICAL server-side copy behavior

---

## Operation 4: SIGN (Generate Signed URLs for Diego)

This is the CRITICAL operation where signed URLs are generated for Diego cells to download droplets.

### OLD: Ruby DAV Client with External Signer (Internal URL)

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│ lib/cloud_controller/blobstore/url_generator/internal_url_generator.rb│
│                                                                 │
│ droplet_download_url(droplet)                                  │
│   │                                                             │
│   ├─ blob = @droplet_blobstore.blob("droplet-guid")            │
│   │  Returns DavBlob with partitioned key: "dr/op/droplet-guid"│
│   │                                                             │
│   └─ blob.internal_download_url  ← CALLED ON-DEMAND (lazy)     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ DavBlob.internal_download_url                                   │
│ lib/cloud_controller/blobstore/webdav/dav_blob.rb             │
│                                                                 │
│ expires = Time.now.utc.to_i + 3600  # 1 hour from now          │
│ @signer.sign_internal_url(path: @key, expires: expires)        │
│   @key = "dr/op/droplet-guid" (partitioned)                    │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ NginxSecureLinkSigner.sign_internal_url                         │
│ lib/cloud_controller/blobstore/webdav/nginx_secure_link_signer.rb│
│                                                                 │
│ Config (from BOSH properties):                                  │
│   @internal_uri = "https://blobstore.service.cf.internal:4443" │
│   @internal_path_prefix = "cc-droplets"  ← NO /admin prefix    │
│   @basic_auth_user = "blobstore-user"                          │
│   @basic_auth_password = "secret123"                            │
│                                                                 │
│ Step 1: Build sign request URI                                 │
│ ─────────────────────────────────────────────────────────────  │
│ path = File.join([@internal_path_prefix, key].compact)         │
│      = "cc-droplets/dr/op/droplet-guid"                        │
│                                                                 │
│ request_uri = uri(expires:, path:)                              │
│ = "https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=/cc-droplets/dr/op/droplet-guid"│
│                                                                 │
│ Step 2: Call external signer                                   │
│ ─────────────────────────────────────────────────────────────  │
│ response = @client.get(request_uri, header: basic_auth_header) │
│ response_uri = URI(response.content)  # Parse response body    │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ GET /sign?expires=1778170942&path=/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore → Blobstore URL Signer Service                 │
│ src/github.com/cloudfoundry/blobstore_url_signer/signer/sign.go│
│                                                                 │
│ func Sign(expire, path string) string {                        │
│   // path = "/cc-droplets/dr/op/droplet-guid"                  │
│   // expire = "1778170942"                                      │
│                                                                 │
│   signature := md5("{expires}/read{path} {secret}")            │
│              = md5("1778170942/read/cc-droplets/dr/op/droplet-guid SECRET")│
│   signature = base64_url_safe(md5sum)                          │
│             = "Xy3aBc..." (sanitized: / → _, + → -, remove =)  │
│                                                                 │
│   return "http://blobstore.service.cf.internal/read{path}?md5={sig}&expires={exp}"│
│        = "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=Xy3aBc...&expires=1778170942"│
│ }                                                               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Returns: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=...
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ NginxSecureLinkSigner (continued)                               │
│                                                                 │
│ Step 3: Replace host with internal endpoint                    │
│ ─────────────────────────────────────────────────────────────  │
│ response_uri = "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=..."│
│                                                                 │
│ signed_uri = @internal_uri.clone  # https://blobstore.service.cf.internal:4443│
│ signed_uri.scheme = 'https'                                     │
│ signed_uri.path = response_uri.path                             │
│ signed_uri.query = response_uri.query                           │
│                                                                 │
│ Final URL:                                                      │
│ "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=Xy3aBc...&expires=1778170942"│
│                                                                 │
│ This URL is returned to CCNG and passed to Diego                │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Signed URL passed to Diego BBS
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Diego Cell (Rep)                                                │
│                                                                 │
│ Downloads droplet when starting app container                   │
│ GET https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=Xy3aBc...&expires=1778170942│
│                                                                 │
│ ✓ TLS verification succeeds (has blobstore_tls.ca cert)        │
│ ✓ No Basic Auth needed (signed URL with MD5 signature)         │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ GET /read/cc-droplets/dr/op/droplet-guid?md5=...&expires=...
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal - Port 4443)                         │
│                                                                 │
│ location /read/ {                                               │
│   secure_link $arg_md5,$arg_expires;                           │
│   secure_link_md5 "$secure_link_expires$uri SECRET";          │
│   # Calculates: md5("1778170942/read/cc-droplets/dr/op/droplet-guid SECRET")│
│   # Compares with md5 query param                               │
│                                                                 │
│   if ($secure_link = "") { return 403; }  # Invalid signature  │
│   if ($secure_link = "0") { return 410; }  # Expired           │
│                                                                 │
│   alias /var/vcap/store/shared/;                               │
│ }                                                               │
│                                                                 │
│ ✓ Signature valid, serves:                                     │
│   /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid        │
└─────────────────────────────────────────────────────────────────┘
```

### NEW: Storage-CLI (CURRENT) - Internal URL

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│ lib/cloud_controller/blobstore/url_generator/internal_url_generator.rb│
│                                                                 │
│ droplet_download_url(droplet)                                  │
│   │                                                             │
│   ├─ blob = @droplet_blobstore.blob("droplet-guid")            │
│   │  Returns StorageCliBlob with:                               │
│   │    @key = "dr/op/droplet-guid" (partitioned)               │
│   │    @storage_cli_client = <StorageCliClient reference>      │
│   │    (Lazy signing enabled for DAV only)                      │
│   │                                                             │
│   └─ blob.internal_download_url  ← CALLED ON-DEMAND (lazy)     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ StorageCliBlob.internal_download_url                            │
│ lib/cloud_controller/blobstore/storage_cli/storage_cli_blob.rb │
│                                                                 │
│ if @storage_cli_client&.supports_lazy_signing?                 │
│   return @storage_cli_client.sign_internal_url(                │
│     @key, verb: 'get', expires_in_seconds: 3600)               │
│ end                                                             │
│ # For non-DAV providers, use pre-generated signed_url           │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ StorageCliClient.sign_internal_url                              │
│ lib/cloud_controller/blobstore/storage_cli/storage_cli_client.rb│
│                                                                 │
│ def sign_internal_url(key, verb:, expires_in_seconds:)         │
│   stdout, _status = run_cli(                                   │
│     'sign-internal',                                            │
│     partitioned_key(key),  # "dr/op/droplet-guid"              │
│     verb.to_s.downcase,    # "get"                              │
│     "#{expires_in_seconds}s"  # "3600s"                         │
│   )                                                             │
│   stdout.strip                                                  │
│ end                                                             │
│                                                                 │
│ Shell command executed:                                         │
│ /var/vcap/packages/storage-cli/bin/storage-cli \               │
│   -s dav \                                                      │
│   -c /var/vcap/jobs/cloud_controller_ng/config/droplets.json \ │
│   sign-internal dr/op/droplet-guid get 3600s                   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Spawns subprocess: storage-cli sign-internal
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storage-cli Binary (Go)                                         │
│ storage/commandexecuter.go                                      │
│                                                                 │
│ Execute("sign-internal", ["dr/op/droplet-guid", "get", "3600s"])│
│   │                                                             │
│   └─ Type assertion: if signer, ok := sty.str.(SignerInternal) │
│      ✓ DavBlobstore implements SignerInternal (optional interface)│
│      ✗ Other providers (S3, Azure, GCS) don't implement it     │
│                                                                 │
│      signer.SignInternal(objectID, action, expiration)          │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ DavBlobstore.SignInternal                                       │
│ dav/client/client.go                                            │
│                                                                 │
│ func (d *DavBlobstore) SignInternal(...) (string, error) {     │
│   return d.storageClient.SignInternal(dest, action, expiration)│
│ }                                                               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storageClient.SignInternal                                      │
│ dav/client/storage_client.go                                    │
│                                                                 │
│ func (c *storageClient) SignInternal(...) (string, error) {    │
│   return c.signWithEndpoint(blobID, action, duration,          │
│     c.config.Endpoint, "internal")                              │
│ }                                                               │
│                                                                 │
│ func (c *storageClient) signWithEndpoint(...) (string, error) {│
│   if c.config.SignedURLFormat == "external-nginx-secure-link-signer" {│
│     return c.signViaExternalEndpoint(blobID, action, duration, endpoint)│
│   }                                                             │
│   // Internal signer (not used for CAPI)                        │
│ }                                                               │
│                                                                 │
│ Config (/var/vcap/jobs/cloud_controller_ng/config/droplets.json):│
│ {                                                               │
│   "provider": "dav",                                            │
│   "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",│
│   "public_endpoint": "https://blobstore.example.com/admin/cc-droplets",│
│   "user": "blobstore-user",                                     │
│   "password": "secret123",                                      │
│   "signed_url_format": "external-nginx-secure-link-signer",    │
│   "tls": { "cert": { "ca": "..." } }                            │
│ }                                                               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storageClient.signViaExternalEndpoint                           │
│ dav/client/storage_client.go                                    │
│                                                                 │
│ Step 1: Extract sign endpoint and directory key                │
│ ─────────────────────────────────────────────────────────────  │
│ // ALWAYS use internal endpoint for calling /sign service      │
│ signEndpoint := extractSignEndpoint(c.config.Endpoint)         │
│   Input:  "https://blobstore.service.cf.internal:4443/admin/cc-droplets"│
│   Output: "https://blobstore.service.cf.internal:4443"         │
│                                                                 │
│ directoryKey := extractDirectoryKey(c.config.Endpoint)         │
│   Input:  "https://blobstore.service.cf.internal:4443/admin/cc-droplets"│
│   Output: "cc-droplets" (strips /admin/)                       │
│                                                                 │
│ Step 2: Build path WITHOUT /admin prefix                       │
│ ─────────────────────────────────────────────────────────────  │
│ signPath := "/" + directoryKey + "/" + blobID                  │
│           = "/cc-droplets/dr/op/droplet-guid"  ← NO /admin!    │
│                                                                 │
│ Step 3: Call external signer                                   │
│ ─────────────────────────────────────────────────────────────  │
│ expires := time.Now().Unix() + int64(duration.Seconds())       │
│          = 1778170942                                           │
│                                                                 │
│ signURL := fmt.Sprintf("%s/sign?expires=%d&path=%s",           │
│   signEndpoint, expires, url.QueryEscape(signPath))            │
│ = "https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=%2Fcc-droplets%2Fdr%2Fop%2Fdroplet-guid"│
│                                                                 │
│ req, _ := http.NewRequest("GET", signURL, nil)                 │
│ req.SetBasicAuth(c.config.User, c.config.Password)             │
│ resp, _ := c.httpClient.Do(req)                                │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ GET /sign?expires=1778170942&path=/cc-droplets/dr/op/droplet-guid
                            │ Authorization: Basic base64(user:pass)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore → Blobstore URL Signer Service                 │
│ [IDENTICAL TO OLD CLIENT - SAME SERVICE]                        │
│                                                                 │
│ Returns: "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=Xy3aBc...&expires=1778170942"│
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Returns signed URL
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storageClient.signViaExternalEndpoint (continued)               │
│                                                                 │
│ Step 4: Replace host with target endpoint (internal)           │
│ ─────────────────────────────────────────────────────────────  │
│ signedURLStr := strings.TrimSpace(string(signedURLBytes))      │
│ = "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=..."│
│                                                                 │
│ responseURL, _ := url.Parse(signedURLStr)                      │
│ targetURL, _ := url.Parse(targetEndpoint)                      │
│ // targetEndpoint = c.config.Endpoint (internal)               │
│ // = "https://blobstore.service.cf.internal:4443/admin/cc-droplets"│
│                                                                 │
│ // Replace scheme and host with target endpoint                │
│ responseURL.Scheme = targetURL.Scheme  // "https"              │
│ responseURL.Host = targetURL.Host      // "blobstore.service.cf.internal:4443"│
│                                                                 │
│ Final URL:                                                      │
│ "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=Xy3aBc...&expires=1778170942"│
│                                                                 │
│ return responseURL.String()                                     │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Prints to stdout
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ StorageCliClient.sign_internal_url (continued)                  │
│                                                                 │
│ stdout = "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=..."│
│ stdout.strip                                                    │
│   ↓                                                             │
│ StorageCliBlob.internal_download_url returns this URL           │
│   ↓                                                             │
│ CCNG passes this URL to Diego                                   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Signed URL passed to Diego BBS
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Diego Cell (Rep)                                                │
│ [IDENTICAL TO OLD CLIENT]                                       │
│                                                                 │
│ GET https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=...│
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                 │
│ [IDENTICAL TO OLD CLIENT - SAME VALIDATION]                     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Operation 5: SIGN PUBLIC (Generate Public Signed URLs)

### OLD: Ruby DAV Client with External Signer (Public URL)

```
┌─────────────────────────────────────────────────────────────────┐
│ User Request via CF API                                         │
│ GET /v3/packages/:guid/download                                 │
│   ↓                                                             │
│ PackagesController → BlobDispatcher                             │
│   ↓                                                             │
│ blob.public_download_url  ← CALLED ON-DEMAND (lazy)            │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ DavBlob.public_download_url                                     │
│                                                                 │
│ expires = Time.now.utc.to_i + 3600                              │
│ @signer.sign_public_url(path: @key, expires: expires)          │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ NginxSecureLinkSigner.sign_public_url                           │
│                                                                 │
│ Config:                                                         │
│   @public_uri = "https://blobstore.example.com"                │
│   @public_path_prefix = "cc-packages"                          │
│                                                                 │
│ Step 1: Call SAME external signer (at internal endpoint)       │
│ ─────────────────────────────────────────────────────────────  │
│ request_uri = "https://blobstore.service.cf.internal:4443/sign?expires=...&path=/cc-packages/pa/ck/package-guid"│
│ response_uri = make_request(uri: request_uri)                   │
│                                                                 │
│ Step 2: Replace host with PUBLIC endpoint (KEY DIFFERENCE!)    │
│ ─────────────────────────────────────────────────────────────  │
│ signed_uri = @public_uri.clone  # https://blobstore.example.com│
│ signed_uri.scheme = 'https'                                     │
│ signed_uri.path = response_uri.path                             │
│ signed_uri.query = response_uri.query                           │
│                                                                 │
│ Final URL:                                                      │
│ "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=..."│
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ CF API redirects user
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ User's Browser                                                  │
│ GET https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=...│
│                                                                 │
│ ✓ TLS verification succeeds (public CA cert)                   │
│ ✓ No Basic Auth needed (signed URL)                            │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Public - Port 443)                            │
│ [SAME VALIDATION, SAME FILES, DIFFERENT HOSTNAME]              │
│                                                                 │
│ ✓ Signature valid, serves:                                     │
│   /var/vcap/store/shared/cc-packages/pa/ck/package-guid        │
└─────────────────────────────────────────────────────────────────┘
```

### NEW: Storage-CLI (CURRENT) - Public URL

```
┌─────────────────────────────────────────────────────────────────┐
│ User Request via CF API                                         │
│ GET /v3/packages/:guid/download                                 │
│   ↓                                                             │
│ PackagesController → BlobDispatcher                             │
│   ↓                                                             │
│ blob.public_download_url  ← CALLED ON-DEMAND (lazy)            │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ StorageCliBlob.public_download_url                              │
│                                                                 │
│ if @storage_cli_client&.supports_lazy_signing?                 │
│   return @storage_cli_client.sign_public_url(                  │
│     @key, verb: 'get', expires_in_seconds: 3600)               │
│ end                                                             │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ StorageCliClient.sign_public_url                                │
│                                                                 │
│ Shell command executed:                                         │
│ /var/vcap/packages/storage-cli/bin/storage-cli \               │
│   -s dav \                                                      │
│   -c /var/vcap/jobs/cloud_controller_ng/config/packages.json \ │
│   sign-public pa/ck/package-guid get 3600s                      │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storage-cli → DavBlobstore.SignPublic                           │
│ dav/client/client.go                                            │
│                                                                 │
│ func (d *DavBlobstore) SignPublic(...) (string, error) {       │
│   return d.storageClient.SignPublic(dest, action, expiration)  │
│ }                                                               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storageClient.SignPublic                                        │
│ dav/client/storage_client.go                                    │
│                                                                 │
│ func (c *storageClient) SignPublic(...) (string, error) {      │
│   // Use public endpoint if configured                          │
│   endpoint := c.config.PublicEndpoint                           │
│   if endpoint == "" {                                           │
│     endpoint = c.config.Endpoint  // fallback                   │
│   }                                                             │
│   return c.signWithEndpoint(blobID, action, duration,          │
│     endpoint, "public")                                         │
│ }                                                               │
│                                                                 │
│ // Calls SAME signViaExternalEndpoint()                         │
│ // BUT passes c.config.PublicEndpoint as targetEndpoint        │
│                                                                 │
│ Config:                                                         │
│   PublicEndpoint = "https://blobstore.example.com/admin/cc-packages"│
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ storageClient.signViaExternalEndpoint                           │
│                                                                 │
│ Step 1-3: IDENTICAL to internal (calls same external signer)   │
│ ─────────────────────────────────────────────────────────────  │
│ signEndpoint = "https://blobstore.service.cf.internal:4443"    │
│ signPath = "/cc-packages/pa/ck/package-guid"                   │
│ Calls: GET /sign?expires=...&path=/cc-packages/pa/ck/...       │
│ Receives: "http://blobstore.service.cf.internal/read/cc-packages/pa/ck/package-guid?md5=...&expires=..."│
│                                                                 │
│ Step 4: Replace host with PUBLIC endpoint (KEY DIFFERENCE!)    │
│ ─────────────────────────────────────────────────────────────  │
│ targetURL = "https://blobstore.example.com/admin/cc-packages"  │
│ responseURL.Scheme = "https"                                    │
│ responseURL.Host = "blobstore.example.com"  ← PUBLIC hostname  │
│                                                                 │
│ Final URL:                                                      │
│ "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=..."│
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Prints to stdout, returned to CCNG
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ CF API redirects user                                           │
│ HTTP/1.1 302 Found                                              │
│ Location: https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=...│
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ User's Browser                                                  │
│ [IDENTICAL TO OLD CLIENT]                                       │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Public - Port 443)                            │
│ [IDENTICAL TO OLD CLIENT]                                       │
└─────────────────────────────────────────────────────────────────┘
```

---

## Summary: OLD vs NEW

### What Stayed EXACTLY the Same

1. **Lazy Signing**
   - OLD: DavBlob calls signer methods on-demand
   - NEW: StorageCliBlob calls storage-cli commands on-demand
   - **Result:** IDENTICAL behavior - URLs generated when needed

2. **External Signer Service**
   - OLD: NginxSecureLinkSigner calls `/sign` endpoint
   - NEW: storage-cli calls `/sign` endpoint
   - **Result:** SAME blobstore_url_signer service, SAME MD5 algorithm

3. **Signed URL Format**
   - OLD: `/read/{directoryKey}/{blobID}?md5=...&expires=...`
   - NEW: `/read/{directoryKey}/{blobID}?md5=...&expires=...`
   - **Result:** IDENTICAL format, IDENTICAL path (NO /admin)

4. **Dual Endpoints**
   - OLD: `private_endpoint` (internal) + `public_endpoint` (public)
   - NEW: `endpoint` (internal) + `public_endpoint` (public)
   - **Result:** SAME concept, SAME two hostnames

5. **Path Construction**
   - OLD: Strips `/admin` before calling signer
   - NEW: Extracts `directoryKey` from endpoint, strips `/admin` before calling signer
   - **Result:** SAME path sent to signer: `/cc-droplets/dr/op/...`

6. **Endpoint Replacement Logic**
   - OLD: NginxSecureLinkSigner replaces host with `@internal_uri` or `@public_uri`
   - NEW: storage-cli replaces host with `config.Endpoint` or `config.PublicEndpoint`
   - **Result:** SAME logic, SAME final URLs

### What Changed (Implementation Only)

| Aspect | OLD | NEW |
|--------|-----|-----|
| **Language** | Pure Ruby | Ruby → Go (subprocess) |
| **Process** | In-process | External binary |
| **Interface** | Method calls | CLI commands |
| **Blob Class** | DavBlob | StorageCliBlob |
| **Client Class** | DavClient | StorageCliClient |
| **Signer** | NginxSecureLinkSigner | storage-cli (Go) |
| **Config Format** | Ruby hash | JSON file |
| **Lazy Signing Check** | Always for WebDAV | `supports_lazy_signing?` returns true for DAV only |
| **Two Signing Methods** | `sign_internal_url` / `sign_public_url` on signer | `sign-internal` / `sign-public` CLI commands |
| **Optional Feature** | N/A (Ruby duck typing) | `SignerInternal` optional interface (Go type assertion) |

### Configuration Comparison

**OLD WebDAV Config (BOSH properties):**
```yaml
webdav_config:
  private_endpoint: "https://blobstore.service.cf.internal:4443"  # NO /admin
  public_endpoint: "https://blobstore.<system-domain>"
  directory_key: "cc-droplets"  # Separate field
  username: "blobstore-user"
  password: "secret123"
```

**NEW storage-cli Config (JSON file):**
```json
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

**Key Differences:**
- OLD: `private_endpoint` + separate `directory_key`
- NEW: Combined in `endpoint` path (extracted by helper functions)
- OLD: Config in Ruby code
- NEW: Config in JSON file
- Both: Support dual endpoints (internal + public)

### URL Flow Comparison

**Internal Signed URL (Diego):**
```
OLD: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=...
NEW: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=...&expires=...
```
✅ **IDENTICAL**

**Public Signed URL (CF API):**
```
OLD: https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=...
NEW: https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=...&expires=...
```
✅ **IDENTICAL**

---

## Conclusion

**The NEW storage-cli implementation maintains 100% behavioral compatibility with the OLD WebDAV client:**

✅ **PUT, GET, DELETE, COPY** - IDENTICAL URLs and behavior
✅ **Internal signing (Diego)** - IDENTICAL signed URLs for internal endpoint
✅ **Public signing (CF API)** - IDENTICAL signed URLs for public endpoint
✅ **Lazy signing** - URLs generated on-demand, NOT pre-generated
✅ **External signer integration** - SAME blobstore_url_signer service
✅ **Path construction** - SAME path format (NO /admin prefix in signed URLs)
✅ **Dual endpoints** - SAME internal/public endpoint logic
✅ **WebDAV-specific** - Other providers (S3, Azure, GCS) unchanged via optional interface

**From the perspective of Diego cells, external users, and nginx blobstore:**
- Nothing changes
- Same signed URL format
- Same MD5 signatures
- Same file paths
- Same TLS endpoints

**The only changes are internal to CCNG:**
- Ruby calls Go binary instead of Ruby code
- JSON config file instead of Ruby hash
- CLI commands instead of method calls
- Optional interface for WebDAV-specific features


The internal MD5 signer (secure-link-md5) is NOT used for CAPI/CF deployments. It's completely covered by the external blobstore_url_signer service.

  Three Signing Methods in storage-cli

  1. hmac-sha256 (default) - Internal HMAC-SHA256 signer
    - Used by: BOSH (when enable_signed_urls: true)
    - Format: /signed/{directoryKey}/{blobID}?st=...&ts=...&e=...
    - Requires: secret in config, NO signed_url_format or signed_url_format: "hmac-sha256"
  2. secure-link-md5 - Internal MD5 signer
    - Used by: Nobody currently (available but not used in practice)
    - Format: /read/{directoryKey}/{blobID}?md5=...&expires=... OR /write/...?md5=...
    - Requires: secret in config, signed_url_format: "secure-link-md5"
  3. external-nginx-secure-link-signer - Calls external signer service
    - Used by: CAPI/CF (all droplets, packages, buildpacks, etc.)
    - Format: Same as MD5 - /read/{directoryKey}/{blobID}?md5=...&expires=...
    - Requires: signed_url_format: "external-nginx-secure-link-signer", NO secret needed
    - Calls: blobstore_url_signer Go service via /sign endpoint

  Code Logic

  // dav/client/storage_client.go - NewStorageClient()

  var urlSigner URLsigner.Signer

  // Only create internal signer if:
  // 1. Secret is provided AND
  // 2. NOT using external signer
  if config.Secret != "" && config.SignedURLFormat != "external-nginx-secure-link-signer" {
      if config.SignedURLFormat != "" {
          signer, err := URLsigner.NewSignerWithFormat(config.Secret, config.SignedURLFormat)
          // Creates either hmac-sha256 or secure-link-md5 internal signer
          urlSigner = signer
      } else {
          urlSigner = URLsigner.NewSigner(config.Secret)  // Default: hmac-sha256
      }
  }

  // Later in signWithEndpoint():
  if c.config.SignedURLFormat == "external-nginx-secure-link-signer" {
      return c.signViaExternalEndpoint(blobID, action, duration, endpoint)
  }

  // Internal signer
  if c.signer == nil {
      return "", fmt.Errorf("signing is not configured (no secret provided)")
  }
  signedURL, err := c.signer.GenerateSignedURL(endpointBase, directoryKey, blobID, action, signTime, duration)

  CAPI/CF Configuration (Current)

  {
    "provider": "dav",
    "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",
    "public_endpoint": "https://blobstore.example.com/admin/cc-droplets",
    "user": "blobstore-user",
    "password": "secret123",
    "signed_url_format": "external-nginx-secure-link-signer",
    "tls": { "cert": { "ca": "..." } }
  }

  Note: NO secret field! Because CAPI uses the external signer service, not internal signing.

  BOSH Configuration (When Using Signed URLs)

  {
    "endpoint": "https://bosh-director-ip:25250",
    "user": "director",
    "password": "secret123",
    "secret": "hmac-signing-secret",
    "signed_url_format": "hmac-sha256"
  }

  Note: HAS secret field! BOSH uses internal HMAC-SHA256 signing (not MD5, not external signer).
  

BOSH Blobstore Usage

  BOSH has its OWN internal blobstore, it does NOT use the external CF/CAPI WebDAV blobstore:

  1. BOSH Director's Internal Blobstore

  # BOSH Director configuration
  blobstore:
    address: ((internal_ip))  # BOSH VM itself
    agent:
      password: ((blobstore_agent_password))
      user: agent
    director:
      password: ((blobstore_director_password))
      user: director
    port: 25250  # Different port (not 4443 like CF)
    provider: dav

  Key differences from CF's blobstore:
  - Port 25250 (not 4443 like CF's internal blobstore)
  - Runs on BOSH Director VM (not a separate blobstore VM)
  - Used for BOSH internal operations (compiled packages, stemcells, releases)
  - NOT accessible from outside BOSH network

  2. BOSH Uses davcli (Not storage-cli)

  BOSH Director uses its own CLI tool called davcli:

  # bosh/src/bosh-director/lib/bosh/director/blobstore/davcli_blobstore_client.rb
  @davcli_path = "/var/vcap/packages/davcli/bin/davcli"

  # Commands:
  Open3.capture3("#{@davcli_path}", '-c', "#{@config_file_path}", 'get', "#{id}", "#{file.path}")
  Open3.capture3("#{@davcli_path}", '-c', "#{@config_file_path}", 'put', "#{content_path}", "#{server_path}")
  Open3.capture3("#{@davcli_path}", '-c', "#{@config_file_path}", 'sign', "#{object_id}", "#{verb}", "#{duration}")

  davcli is a separate binary (not storage-cli):
  - Located at /var/vcap/packages/davcli/bin/davcli
  - Used by BOSH Director
  - Has sign command for pre-signed URLs

  3. BOSH Signed URLs (Optional)

  BOSH supports signed URLs via the blobstore.enable_signed_urls property:

  blobstore:
    enable_signed_urls: false  # Default is false
    secret: "signing-secret"   # Used for HMAC signatures

  When enabled:
  - BOSH VMs download compiled packages via signed URLs
  - No blobstore credentials needed on VMs
  - Uses internal HMAC-SHA256 signing (not external signer service)

  4. CF CAPI Uses External WebDAV Blobstore

  When CF is deployed, CAPI configures its OWN separate blobstore:

  # CF deployment manifest
  cc:
    droplets:
      storage_cli_config_file_droplets: /var/vcap/jobs/cloud_controller_ng/config/droplets.json

  # droplets.json content:
  {
    "provider": "dav",
    "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",
    "signed_url_format": "external-nginx-secure-link-signer"
  }

  This is SEPARATE from BOSH's blobstore:
  - Port 4443 (CF's blobstore)
  - Runs on separate blobstore VM
  - Used for droplets/buildpacks/packages
  - Uses external signer service (blobstore_url_signer)

  ---
  Summary

  ┌─────────────────┬─────────────────────────────────────────────┬────────────────────────────────────────────────┐
  │     Aspect      │               BOSH Blobstore                │               CF/CAPI Blobstore                │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Tool            │ davcli                                      │ storage-cli (NEW) / WebDAV client (OLD)        │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Port            │ 25250                                       │ 4443 (internal) / 443 (public)                 │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Location        │ BOSH Director VM                            │ Separate blobstore VM                          │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Purpose         │ BOSH releases, compiled packages, stemcells │ CF droplets, buildpacks, packages              │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Signing         │ Internal HMAC-SHA256 (optional)             │ External signer service (blobstore_url_signer) │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Users           │ BOSH VMs (when deploying)                   │ Diego cells, CF API users                      │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ External Access │ No                                          │ Yes (public endpoint for CF API)               │
  ├─────────────────┼─────────────────────────────────────────────┼────────────────────────────────────────────────┤
  │ Endpoint Config │ Single endpoint                             │ Dual endpoints (internal + public)             │
  └─────────────────┴─────────────────────────────────────────────┴────────────────────────────────────────────────┘

  Answer: No, BOSH does NOT use the external CF WebDAV blobstore. BOSH has its own internal blobstore running on port 25250 on the BOSH Director VM.
  

