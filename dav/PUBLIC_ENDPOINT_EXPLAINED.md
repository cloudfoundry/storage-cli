# Why Old WebDAV Implementation Had public_endpoint

## TL;DR
**Public endpoint was for user-facing downloads through Cloud Foundry API (e.g., `cf download-droplet`). Internal endpoint was for Diego cells downloading during staging/running apps.**

## Two Different Download Paths in Cloud Foundry

### 1. **Diego Cell Downloads** (Internal - `sign_internal_url`)
- **Who:** Diego cells (staging containers, app instances)
- **When:** During app staging, starting apps, downloading buildpacks/droplets
- **URL Used:** `internal_download_url` → `sign_internal_url` → **internal endpoint**
- **Network:** Internal Cloud Foundry network
- **TLS Cert:** Internal CA (known to Diego cells)
- **Example Flow:**
  ```
  Diego Cell → CAPI → UrlGenerator.droplet_download_url()
            → blob.internal_download_url
            → signer.sign_internal_url(path: blob_key)
            → blobstore_url_signer /sign endpoint
            → Returns: https://blobstore.service.cf.internal:4443/read/cc-droplets/...?md5=...
  ```

### 2. **User/API Downloads** (Public - `sign_public_url`)
- **Who:** External users via CF API (cf CLI, browser)
- **When:** User runs `cf download-droplet`, downloads buildpack via API
- **URL Used:** `public_download_url` → `sign_public_url` → **public endpoint**
- **Network:** Public internet / external load balancer
- **TLS Cert:** Public CA (trusted by browsers/cf CLI)
- **Example Flow:**
  ```
  User → CF API endpoint (GET /v3/packages/:guid/download)
       → BlobDispatcher.send_or_redirect()
       → blob.public_download_url
       → signer.sign_public_url(path: blob_key)
       → blobstore_url_signer /sign endpoint
       → Returns: https://blobstore.cf.example.com/read/cc-packages/...?md5=...
       → API responds with HTTP 302 redirect to public URL
  ```

## Code Evidence

### Old WebDAV Implementation

**Two signing methods in `nginx_secure_link_signer.rb`:**

```ruby
def sign_internal_url(expires:, path:)
  request_uri  = uri(expires: expires, path: File.join([@internal_path_prefix, path].compact))
  response_uri = make_request(uri: request_uri)
  
  signed_uri        = @internal_uri.clone
  signed_uri.scheme = 'https'
  signed_uri.path   = response_uri.path
  signed_uri.query  = response_uri.query
  signed_uri.to_s  # Returns URL with INTERNAL endpoint host
end

def sign_public_url(expires:, path:)
  request_uri  = uri(expires: expires, path: File.join([@public_path_prefix, path].compact))
  response_uri = make_request(uri: request_uri)
  
  signed_uri        = @public_uri.clone  # NOTE: Uses public_uri!
  signed_uri.scheme = 'https'
  signed_uri.path   = response_uri.path
  signed_uri.query  = response_uri.query
  signed_uri.to_s  # Returns URL with PUBLIC endpoint host
end
```

**Both call the same `/sign` endpoint on internal blobstore, but replace the host differently.**

### Blob Implementation

**`dav_blob.rb`:**
```ruby
def internal_download_url
  expires = Time.now.utc.to_i + 3600
  @signer.sign_internal_url(path: @key, expires: expires)
end

def public_download_url
  expires = Time.now.utc.to_i + 3600
  @signer.sign_public_url(path: @key, expires: expires)
end
```

### Usage in CAPI

**Diego Cell Downloads (`internal_download_url`):**
```ruby
# lib/cloud_controller/blobstore/url_generator/internal_url_generator.rb
def droplet_download_url(droplet)
  blob = @droplet_blobstore.blob(droplet.blobstore_key)
  url_for_blob(blob)  # Returns blob.internal_download_url
end

# lib/cloud_controller/diego/lifecycle_protocol.rb
lifecycle_data.app_bits_download_uri = @blobstore_url_generator.package_download_url(staging_details.package)
# This URL goes into Diego staging task → Diego cells use it
```

**User Downloads (`public_download_url`):**
```ruby
# app/controllers/runtime/helpers/blob_dispatcher.rb
def send_or_redirect_blob(blob)
  if @blobstore.local?
    blob_sender.send_blob(blob, @controller)  # X-Accel-Redirect with internal_download_url
  else
    @controller.redirect blob.public_download_url  # HTTP 302 to public endpoint
  end
end

# app/controllers/v3/packages_controller.rb
def download
  # ... authorization checks ...
  BlobDispatcher.new(blobstore: package_blobstore, controller: self).send_or_redirect(guid: package.guid)
  # User's browser gets redirected to public_download_url
end
```

## Why Two Endpoints?

### Network Architecture Reasons:

1. **Security Isolation**
   - Internal endpoint only accessible within CF network
   - Public endpoint exposed through load balancer
   - Prevents Diego cells from needing public internet access

2. **Certificate Management**
   - Internal: Self-signed or internal CA (trusted by CF components)
   - Public: Public CA cert (trusted by user browsers/CLI)
   - Diego cells configured with internal CA only

3. **DNS Resolution**
   - Internal: `blobstore.service.cf.internal` (service discovery)
   - Public: `blobstore.cf.example.com` (public DNS)
   - Diego cells only resolve internal DNS

4. **Load Balancing**
   - Internal: Direct connection to blobstore (high throughput)
   - Public: Through load balancer (rate limiting, DDoS protection)

## Why We Removed public_endpoint in storage-cli

### The Key Realization:
**Old WebDAV client supported BOTH internal and public downloads, but storage-cli ONLY handles internal downloads (Diego path).**

### Why?

1. **User Downloads Changed:**
   - Modern CAPI uses **direct blobstore API access** for user downloads
   - No longer redirects users to blobstore URLs
   - Users download through Cloud Controller, which proxies from blobstore

2. **storage-cli Scope:**
   - storage-cli is only used by **Cloud Controller internal operations**
   - Diego cells get URLs from Cloud Controller's internal URL generator
   - User-facing downloads don't go through storage-cli Sign() function

3. **Simplified Model:**
   - storage-cli Sign() → Always returns internal endpoint URLs
   - These URLs only used by Diego cells (internal network)
   - No need for public_endpoint configuration

## Configuration Comparison

### Old WebDAV Client (Ruby)
```yaml
blobstore:
  private_endpoint: https://blobstore.service.cf.internal:4443
  public_endpoint: https://blobstore.cf.example.com   # Used for user downloads
  username: admin
  password: secret
  ca_cert: |
    -----BEGIN CERTIFICATE-----
    ... (internal CA)
```

### storage-cli (Go)
```json
{
  "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets",
  "user": "admin",
  "password": "secret",
  "secret": "signing-secret",
  "signed_url_format": "external-nginx-secure-link-signer",
  "tls": {
    "cert": {
      "ca": "-----BEGIN CERTIFICATE-----\n... (internal CA)"
    }
  }
}
```

**No public_endpoint needed** - storage-cli only generates internal URLs for Diego.

## Summary Table

| Aspect | Internal Endpoint | Public Endpoint |
|--------|------------------|-----------------|
| **Used By** | Diego cells | External users (cf CLI, browser) |
| **Network** | Internal CF network | Public internet |
| **DNS** | `blobstore.service.cf.internal` | `blobstore.cf.example.com` |
| **TLS Cert** | Internal CA | Public CA |
| **Access Path** | Direct to blobstore | Through load balancer |
| **Old WebDAV** | `sign_internal_url()` | `sign_public_url()` |
| **storage-cli** | `Sign()` always returns internal | Not supported (not needed) |
| **URL Format** | `/read/cc-droplets/...?md5=...` | Same path, different host |

## Why It Worked Before Without Issues

In the old Ruby WebDAV client:
- `BlobDispatcher` used `public_download_url` for user downloads
- `InternalUrlGenerator` used `internal_download_url` for Diego downloads
- Two separate code paths maintained

In storage-cli:
- **User downloads no longer use signed URL redirects** (changed in CAPI)
- Only Diego downloads use storage-cli Sign()
- One code path needed (internal only)

## Conclusion

**public_endpoint was for user-facing downloads that went through HTTP redirects to the blobstore. This pattern is no longer used in modern CAPI - users download through Cloud Controller proxy, not direct blobstore redirects. storage-cli only needs to support internal endpoint for Diego cell downloads.**
