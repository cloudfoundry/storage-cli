# OLD WebDAV Client Signing Flow - Detailed Analysis

## Overview

The OLD WebDAV client (`DavClient` + `DavBlob` + `NginxSecureLinkSigner`) supports **TWO SEPARATE signing methods**:
1. `sign_internal_url` - For Diego cells (internal network)
2. `sign_public_url` - For external users via CF API

**CRITICAL:** These methods are called **ON DEMAND** when needed, NOT pre-generated and cached.

---

## Architecture Components

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                         │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐ │
│  │ DavClient    │───>│ DavBlob      │───>│ NginxSecureLink  │ │
│  │              │    │              │    │ Signer           │ │
│  │ - blob(key)  │    │ - @signer    │    │ - sign_internal  │ │
│  │              │    │ - @key       │    │ - sign_public    │ │
│  └──────────────┘    └──────────────┘    └──────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Configuration Setup

### BOSH Manifest (cf-deployment)

```yaml
# Droplets blobstore config
cc:
  droplets:
    droplet_directory_key: cc-droplets
    blobstore_provider: webdav
    webdav_config:
      private_endpoint: https://blobstore.service.cf.internal:4443
      public_endpoint: https://blobstore.example.com
      username: blobstore-user
      password: secret123
      ca_cert: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
```

### DavClient Initialization

```ruby
# lib/cloud_controller/blobstore/client_provider.rb
DavClient.build(
  options,                    # from webdav_config above
  'cc-droplets',             # directory_key
  nil,                       # root_dir
  0,                         # min_size
  nil                        # max_size
)
```

**DavClient.build creates:**

```ruby
DavClient.new(
  directory_key: 'cc-droplets',
  httpclient: <HTTP client with CA cert>,
  signer: NginxSecureLinkSigner.new(
    internal_endpoint: 'https://blobstore.service.cf.internal:4443',
    internal_path_prefix: 'cc-droplets',
    public_endpoint: 'https://blobstore.example.com',
    public_path_prefix: 'cc-droplets',
    basic_auth_user: 'blobstore-user',
    basic_auth_password: 'secret123',
    httpclient: <HTTP client>
  ),
  endpoint: 'https://blobstore.service.cf.internal:4443',
  user: 'blobstore-user',
  password: 'secret123'
)
```

**Key point:** The signer is initialized with BOTH endpoints:
- `@internal_uri` = `https://blobstore.service.cf.internal:4443`
- `@public_uri` = `https://blobstore.example.com`

---

## Flow 1: Diego Downloads (Internal)

### Step 1: CCNG Prepares Staging Task

```ruby
# lib/cloud_controller/diego/buildpack/staging_action_builder.rb
# or similar staging/running code

# Get blob for droplet
blob = droplet_blobstore.blob(droplet.guid)
# blob is a DavBlob instance with @signer reference

# Generate internal download URL for Diego
download_url = blob.internal_download_url
```

### Step 2: DavBlob.internal_download_url

```ruby
# lib/cloud_controller/blobstore/webdav/dav_blob.rb

def internal_download_url
  expires = Time.now.utc.to_i + 3600  # 1 hour from now
  @signer.sign_internal_url(path: @key, expires: expires)
end
```

**Input:**
- `@key` = `"dr/op/droplet-guid"` (partitioned key)
- `expires` = `1778170942` (Unix timestamp)

### Step 3: NginxSecureLinkSigner.sign_internal_url

```ruby
# lib/cloud_controller/blobstore/webdav/nginx_secure_link_signer.rb

def sign_internal_url(expires:, path:)
  # Build path with directory key
  request_uri = uri(
    expires: expires,
    path: File.join([@internal_path_prefix, path].compact)
  )
  # path = "cc-droplets/dr/op/droplet-guid"
  
  # Call external signer
  response_uri = make_request(uri: request_uri)
  
  # Replace host with internal endpoint
  signed_uri        = @internal_uri.clone
  signed_uri.scheme = 'https'
  signed_uri.path   = response_uri.path
  signed_uri.query  = response_uri.query
  signed_uri.to_s
end

private

def uri(expires:, path:)
  uri       = @internal_uri.clone  # https://blobstore.service.cf.internal:4443
  uri.path  = '/sign'
  uri.query = {
    expires: expires,               # 1778170942
    path: File.join(['/', path])   # "/cc-droplets/dr/op/droplet-guid"
  }.to_query
  
  # Result: "https://blobstore.service.cf.internal:4443/sign?expires=1778170942&path=%2Fcc-droplets%2Fdr%2Fop%2Fdroplet-guid"
  uri.to_s
end

def make_request(uri:)
  response = @client.get(uri, header: @headers)  # Basic Auth header
  raise SigningRequestError unless response.status == 200
  URI(response.content)  # Parse response body as URI
end
```

### Step 4: External Signer Service

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
    // Returns: "http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942"
}
```

**Response:**
```
200 OK
Content-Type: text/plain

http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942
```

### Step 5: NginxSecureLinkSigner Replaces Host

```ruby
# Received from signer
response_uri = URI("http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942")

# Clone internal endpoint
signed_uri = @internal_uri.clone  # https://blobstore.service.cf.internal:4443

# Replace path and query
signed_uri.scheme = 'https'
signed_uri.path   = response_uri.path   # "/read/cc-droplets/dr/op/droplet-guid"
signed_uri.query  = response_uri.query  # "md5=XYZ123&expires=1778170942"

# Final result
signed_uri.to_s
# => "https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ123&expires=1778170942"
```

### Step 6: Diego Uses Signed URL

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
  # blob is a DavBlob instance
  
  if @blobstore.local?
    blob_sender.send_blob(blob, @controller)
  else
    @controller.redirect blob.public_download_url  # ← CALLS public_download_url!
  end
end
```

### Step 4: DavBlob.public_download_url

```ruby
# lib/cloud_controller/blobstore/webdav/dav_blob.rb

def public_download_url
  expires = Time.now.utc.to_i + 3600  # 1 hour from now
  @signer.sign_public_url(path: @key, expires: expires)  # ← Different method!
end
```

**Input:**
- `@key` = `"pa/ck/package-guid"` (partitioned key)
- `expires` = `1778170942` (Unix timestamp)

### Step 5: NginxSecureLinkSigner.sign_public_url

```ruby
# lib/cloud_controller/blobstore/webdav/nginx_secure_link_signer.rb

def sign_public_url(expires:, path:)
  # Build path with directory key
  request_uri = uri(
    expires: expires,
    path: File.join([@public_path_prefix, path].compact)
  )
  # path = "cc-packages/pa/ck/package-guid"
  
  # Call external signer (SAME service, SAME request)
  response_uri = make_request(uri: request_uri)
  
  # Replace host with PUBLIC endpoint (THIS IS THE KEY DIFFERENCE!)
  signed_uri        = @public_uri.clone  # https://blobstore.example.com
  signed_uri.scheme = 'https'
  signed_uri.path   = response_uri.path
  signed_uri.query  = response_uri.query
  signed_uri.to_s
end
```

**External signer returns:**
```
http://blobstore.service.cf.internal/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942
```

**NginxSecureLinkSigner replaces host:**
```ruby
# Received from signer
response_uri = URI("http://blobstore.service.cf.internal/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942")

# Clone PUBLIC endpoint
signed_uri = @public_uri.clone  # https://blobstore.example.com

# Replace path and query
signed_uri.path   = response_uri.path   # "/read/cc-packages/pa/ck/package-guid"
signed_uri.query  = response_uri.query  # "md5=ABC456&expires=1778170942"

# Final result
signed_uri.to_s
# => "https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942"
```

### Step 6: CF API Redirects User

```
HTTP/1.1 302 Found
Location: https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942
```

### Step 7: User's Browser Downloads

```
User's browser follows redirect:
GET https://blobstore.example.com/read/cc-packages/pa/ck/package-guid?md5=ABC456&expires=1778170942

✓ TLS verification succeeds (public CA cert)
✓ No Basic Auth needed (signed URL)
✓ Nginx validates MD5 signature
✓ Returns file content
```

---

## Key Observations

### 1. Lazy Signing (On-Demand)

**DavBlob does NOT pre-generate signed URLs!**

```ruby
class DavBlob
  def internal_download_url
    # Called when needed (e.g., Diego staging)
    @signer.sign_internal_url(...)
  end
  
  def public_download_url
    # Called when needed (e.g., API download)
    @signer.sign_public_url(...)
  end
end
```

**When are these called?**
- `internal_download_url`: When CCNG prepares a Diego staging/running task
- `public_download_url`: When BlobDispatcher handles API download requests

**They are NEVER called at the same time for the same blob!**

### 2. Same Signer, Different Endpoint Replacement

Both methods:
1. Call the SAME external signer service at the SAME internal endpoint
2. Receive the SAME format response (with `blobstore.service.cf.internal` host)
3. **Differ ONLY in which endpoint they use to replace the host**

```ruby
# Internal
signed_uri = @internal_uri.clone  # https://blobstore.service.cf.internal:4443
signed_uri.path = response_uri.path
signed_uri.query = response_uri.query

# Public
signed_uri = @public_uri.clone  # https://blobstore.example.com
signed_uri.path = response_uri.path
signed_uri.query = response_uri.query
```

**The MD5 signature is the SAME** because it's calculated from the PATH, not the HOST:
```go
signature := md5(fmt.Sprintf("%s/read%s %s", expire, path, secret))
// path = "/cc-droplets/dr/op/droplet-guid" (no host!)
```

So the signed URL `?md5=...&expires=...` works for BOTH endpoints as long as they route to the same nginx with the same secret!

### 3. Two Endpoints, Two Use Cases

| Endpoint | Used For | Accessible From | TLS Cert | Nginx Location |
|----------|----------|----------------|----------|----------------|
| `blobstore.service.cf.internal:4443` | Diego cells | Internal network | Internal CA | Same nginx |
| `blobstore.example.com` | External users | Public internet | Public CA | Same nginx (via load balancer) |

Both endpoints:
- Route to the SAME nginx blobstore instance
- Use the SAME `secure_link_md5` secret
- Serve files from the SAME `/var/vcap/store/shared/` directory
- Accept the SAME `/read/{path}?md5=...&expires=...` format

**The ONLY difference is the hostname and TLS certificate!**

---

## Summary

**The OLD WebDAV client's two signing methods are NOT about different signing algorithms or secrets.**

They are about **which endpoint hostname to put in the final signed URL**:

1. **sign_internal_url**: Returns `https://blobstore.service.cf.internal:4443/read/...?md5=...`
   - For Diego cells (internal network, internal CA cert)

2. **sign_public_url**: Returns `https://blobstore.example.com/read/...?md5=...`
   - For external users (public internet, public CA cert)

**Both URLs:**
- Have the SAME path: `/read/cc-droplets/dr/op/droplet-guid`
- Have the SAME query params: `?md5=XYZ&expires=1778170942`
- Are generated by calling the SAME external signer service
- Work with the SAME nginx blobstore (just accessed via different hostnames)

**The key insight:** The signing is done LAZILY (on-demand) when `blob.internal_download_url` or `blob.public_download_url` is called, NOT eagerly when `blob()` is created!
