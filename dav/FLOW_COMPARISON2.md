# WebDAV Flow Comparison: Old Client vs Storage-CLI

## 1. PUT Operation (Upload)

### OLD WebDAV Client (Ruby)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  1. App upload request comes in                                        │
│  2. BlobstoreClient.cp_to_blobstore(file, "droplet-guid")            │
│     └─> DavClient.create_file("droplet-guid", file_content)          │
│         └─> Builds URL from config:                                   │
│             - private_endpoint: https://blobstore.service.cf.internal:4443 │
│             - directory_key: cc-droplets                              │
│             - Final URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid │
│         └─> HTTP PUT with Basic Auth (username/password)             │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ PUT /admin/cc-droplets/dr/op/droplet-guid
                                     │ Authorization: Basic base64(user:pass)
                                     │ Body: file content
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal Server - Port 4443)                          │
│                                                                         │
│  location /admin/ {                                                    │
│    auth_basic "Blobstore Admin";                                      │
│    auth_basic_user_file write_users;                                  │
│    dav_methods DELETE PUT COPY;                                       │
│    create_full_put_path on;                                           │
│    alias /var/vcap/store/shared/;                                     │
│  }                                                                     │
│                                                                         │
│  File stored at: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────────────┘
```

### NEW Storage-CLI

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  1. App upload request comes in                                        │
│  2. StorageCliClient.cp_to_blobstore(file, "droplet-guid")           │
│     └─> Partitions key: "droplet-guid" → "dr/op/droplet-guid"        │
│     └─> Runs CLI: storage-cli -s dav -c config.json put file.tgz dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Storage-CLI Binary (dav/client/storage_client.go)                      │
│                                                                         │
│  Config loaded from JSON:                                              │
│  {                                                                      │
│    "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets", │
│    "user": "blobstore-user",                                           │
│    "password": "secret"                                                │
│  }                                                                      │
│                                                                         │
│  Put("dr/op/droplet-guid", fileReader, size)                          │
│  └─> createReq("PUT", "dr/op/droplet-guid", body)                    │
│      └─> Builds URL: endpoint + "/" + blobID                          │
│          = https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid │
│      └─> Sets Basic Auth header                                       │
│      └─> HTTP PUT request                                             │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ PUT /admin/cc-droplets/dr/op/droplet-guid
                                     │ Authorization: Basic base64(user:pass)
                                     │ Body: file content
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal Server - Port 4443)                          │
│                                                                         │
│  Same nginx config as old client                                       │
│  File stored at: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key differences:**
- Old: Ruby code directly makes HTTP request
- New: CLI binary spawned as subprocess, makes HTTP request
- **Result: IDENTICAL URLs and behavior**

---

## 2. GET Operation (Download via Basic Auth)

### OLD WebDAV Client

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  DavClient.download_from_blobstore("droplet-guid", local_path)        │
│  └─> URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid │
│  └─> HTTP GET with Basic Auth                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ GET /admin/cc-droplets/dr/op/droplet-guid
                                     │ Authorization: Basic base64(user:pass)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                         │
│  Returns file from: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────────────┘
```

### NEW Storage-CLI

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  StorageCliClient.download_from_blobstore("droplet-guid", local_path) │
│  └─> Partitions: "droplet-guid" → "dr/op/droplet-guid"               │
│  └─> Runs: storage-cli -s dav -c config.json get dr/op/droplet-guid local_path │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Storage-CLI Binary                                                      │
│                                                                         │
│  Get("dr/op/droplet-guid")                                             │
│  └─> URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/dr/op/droplet-guid │
│  └─> HTTP GET with Basic Auth                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ GET /admin/cc-droplets/dr/op/droplet-guid
                                     │ Authorization: Basic base64(user:pass)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                         │
│  Returns file from: /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key differences:**
- **Result: IDENTICAL URLs and behavior**

---

## 3. COPY Operation

### OLD WebDAV Client

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  DavClient.cp_file_between_keys("source-guid", "dest-guid")           │
│  └─> Source URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/so/ur/source-guid │
│  └─> Dest URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid │
│  └─> HTTP COPY request                                                │
│      - Method: COPY                                                    │
│      - URL: source URL                                                 │
│      - Header: Destination: dest URL                                   │
│      - Authorization: Basic Auth                                       │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ COPY /admin/cc-droplets/so/ur/source-guid
                                     │ Destination: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid
                                     │ Authorization: Basic base64(user:pass)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                         │
│  Copies file server-side (no download/upload)                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### NEW Storage-CLI

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  StorageCliClient.cp_file_between_keys("source-guid", "dest-guid")    │
│  └─> Partitions keys                                                   │
│  └─> Runs: storage-cli -s dav -c config.json copy so/ur/source-guid de/st/dest-guid │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Storage-CLI Binary                                                      │
│                                                                         │
│  Copy("so/ur/source-guid", "de/st/dest-guid")                         │
│  └─> copyNative()                                                      │
│      └─> Source URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/so/ur/source-guid │
│      └─> Dest URL: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid │
│      └─> HTTP COPY request (identical to old client)                  │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ COPY /admin/cc-droplets/so/ur/source-guid
                                     │ Destination: https://blobstore.service.cf.internal:4443/admin/cc-droplets/de/st/dest-guid
                                     │ Authorization: Basic base64(user:pass)
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore                                                         │
│  Copies file server-side                                               │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key differences:**
- **Result: IDENTICAL URLs and behavior**

---

## 4. SIGN Operation (For Diego Downloads)

This is the CRITICAL one where the bug was!

### OLD WebDAV Client (with external-nginx-secure-link-signer)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG) - InternalUrlGenerator                         │
│                                                                         │
│  blob.internal_download_url                                            │
│  └─> DavBlob.internal_download_url                                    │
│      └─> NginxSecureLinkSigner.sign_internal_url(path: "dr/op/droplet-guid") │
│          Config:                                                        │
│          - @internal_uri = "https://blobstore.service.cf.internal:4443" │
│          - @internal_path_prefix = "cc-droplets"                       │
│                                                                         │
│          Step 1: Call external signer                                  │
│          ────────────────────────────────────────────────────────────  │
│          Request to blobstore_url_signer service:                      │
│          GET https://blobstore.service.cf.internal:4443/sign           │
│              ?expires=1778170942                                       │
│              &path=/cc-droplets/dr/op/droplet-guid  ◄── INCLUDES DIRECTORY KEY │
│          Authorization: Basic base64(user:pass)                        │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Blobstore URL Signer Service (blobstore_url_signer/signer/sign.go)    │
│                                                                         │
│  func Sign(expire, path string) string {                              │
│    // path = "/cc-droplets/dr/op/droplet-guid"                        │
│    signature := generateSignature(                                     │
│      fmt.Sprintf("%s/read%s %s", expire, path, secret))              │
│    return fmt.Sprintf(                                                 │
│      "http://blobstore.service.cf.internal/read%s?md5=%s&expires=%s", │
│      path, signature, expire)                                          │
│  }                                                                     │
│                                                                         │
│  Returns: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ Returns signed URL
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller - NginxSecureLinkSigner (continued)                   │
│                                                                         │
│          Step 2: Replace host with internal endpoint                   │
│          ────────────────────────────────────────────────────────────  │
│          Takes: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?... │
│          Replaces scheme + host with @internal_uri                     │
│          Result: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
│                                                                         │
│  Returns this URL to Diego                                             │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ Signed URL stored in BBS
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Diego Cell (Rep)                                                        │
│                                                                         │
│  Downloads droplet for app staging/running                             │
│  GET https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
│  No authentication needed (signed URL with MD5 signature)              │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ GET /read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal Server - Port 4443)                          │
│                                                                         │
│  location /read/ {                                                     │
│    secure_link $arg_md5,$arg_expires;                                 │
│    secure_link_md5 "$secure_link_expires$uri SECRET";                │
│    if ($secure_link = "") { return 403; }  # Invalid signature        │
│    if ($secure_link = "0") { return 410; }  # Expired                 │
│    alias /var/vcap/store/shared/;                                     │
│  }                                                                     │
│                                                                         │
│  Verifies MD5 signature, then serves file from:                        │
│  /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid                │
└─────────────────────────────────────────────────────────────────────────┘
```

### NEW Storage-CLI (AFTER our fix - WORKING!)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Cloud Controller (CCNG)                                                 │
│                                                                         │
│  StorageCliClient.sign_url("dr/op/droplet-guid", verb: 'get', ...)   │
│  └─> Runs: storage-cli -s dav -c config.json sign dr/op/droplet-guid get 3600s │
│                                                                         │
│  Config JSON (public_endpoint removed):                                 │
│  {                                                                      │
│    "endpoint": "https://blobstore.service.cf.internal:4443/admin/cc-droplets", │
│    "user": "blobstore-user",                                           │
│    "password": "secret",                                               │
│    "secret": "signing-secret",                                         │
│    "signed_url_format": "external-nginx-secure-link-signer"           │
│  }                                                                      │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Storage-CLI Binary (AFTER FIX - WORKING!)                              │
│                                                                         │
│  Sign("dr/op/droplet-guid", "GET", 3600s)                             │
│  └─> signViaExternalEndpoint()                                        │
│      Step 1: Extract sign endpoint                                     │
│      extractSignEndpoint() → "https://blobstore.service.cf.internal:4443" │
│                                                                         │
│      ✅ FIX 1: Extract directory key!                                  │
│      extractDirectoryKey() → "cc-droplets"                             │
│      (extracted from endpoint path: "/admin/cc-droplets")              │
│                                                                         │
│      path = "/" + directoryKey + "/" + blobID                         │
│           = "/cc-droplets/dr/op/droplet-guid"  ◄── CORRECT!           │
│                                                                         │
│      Step 2: Call external signer                                      │
│      GET https://blobstore.service.cf.internal:4443/sign               │
│          ?expires=1778170942                                           │
│          &path=/cc-droplets/dr/op/droplet-guid  ◄── CORRECT!          │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Blobstore URL Signer Service                                           │
│                                                                         │
│  Signs path: "/cc-droplets/dr/op/droplet-guid"  ◄── CORRECT!          │
│  Returns: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Storage-CLI (continued)                                                 │
│                                                                         │
│      Step 3: Replace host                                              │
│      ✅ FIX 2: Use internal Endpoint instead of PublicEndpoint!        │
│      Takes: http://blobstore.service.cf.internal/read/cc-droplets/dr/op/droplet-guid?... │
│      Replaces with: config.Endpoint = "https://blobstore.service.cf.internal:4443" │
│      Result: https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
│               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^                    │
│               INTERNAL endpoint - Diego cells have this CA cert!       │
│                                                                         │
│  Returns this CORRECT URL to Cloud Controller                          │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ Signed URL stored in BBS
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Diego Cell (Rep)                                                        │
│                                                                         │
│  Downloads droplet:                                                     │
│  GET https://blobstore.service.cf.internal:4443/read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942 │
│                                                                         │
│  ✅ SUCCESS: TLS certificate verification works!                       │
│  Diego cells trust the internal endpoint's CA (blobstore_tls.ca)      │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     │ GET /read/cc-droplets/dr/op/droplet-guid?md5=XYZ&expires=1778170942
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Nginx Blobstore (Internal Server - Port 4443)                          │
│                                                                         │
│  location /read/ {                                                     │
│    secure_link $arg_md5,$arg_expires;                                 │
│    secure_link_md5 "$secure_link_expires$uri SECRET";                │
│    if ($secure_link = "") { return 403; }                             │
│    if ($secure_link = "0") { return 410; }                            │
│    alias /var/vcap/store/shared/;                                     │
│  }                                                                     │
│                                                                         │
│  ✅ Signature valid, serves file from:                                │
│  /var/vcap/store/shared/cc-droplets/dr/op/droplet-guid  ◄── CORRECT! │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Summary of Fixes

### Two Critical Bugs Fixed:

1. **Missing Directory Key in Sign Path**
   - **Bug**: `signViaExternalEndpoint()` passed `/dr/op/droplet-guid` to signer
   - **Fix**: Added `extractDirectoryKey()` to extract `cc-droplets` from endpoint
   - **Result**: Now passes `/cc-droplets/dr/op/droplet-guid` (correct path)

2. **Wrong Endpoint in Signed URL**
   - **Bug**: `replaceHostInURL()` used `PublicEndpoint` (public HTTPS endpoint)
   - **Fix**: Always use `Endpoint` (internal endpoint) for Diego downloads
   - **Result**: Diego cells can verify TLS and access files

### Configuration Changes:

**Removed from config:**
- `public_endpoint` field (no longer used)

**Removed from CAPI templates:**
- `public_endpoint` configuration (8 template files updated)

**Removed from manifest:**
- `public_endpoint: https://blobstore.cf.leia.env.bndl.sapcloud.io` (12 occurrences)

### Why This Matches Old Behavior:

The old WebDAV client's `sign_internal_url` method:
1. Prepended directory key (`@internal_path_prefix`) before calling signer ✓
2. Replaced host with internal URI (`@internal_uri`) after signing ✓

Storage-CLI now does the same:
1. Prepends directory key (`extractDirectoryKey()`) before calling signer ✓
2. Replaces host with internal endpoint (`c.config.Endpoint`) after signing ✓

**Result: IDENTICAL behavior to old WebDAV client!**
