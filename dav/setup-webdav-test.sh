#!/bin/bash
# WebDAV Test Server Setup Script

set -e

echo "=== Setting up WebDAV Test Server with Self-Signed Certificate ==="

# Create test directory structure
mkdir -p webdav-test/{data,certs,config}
cd webdav-test

# Generate self-signed certificate with SAN
echo "1. Generating self-signed certificate..."
cat > certs/openssl.cnf <<'SSLEOF'
[req]
default_bits = 2048
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = Test
L = Test
O = Test
CN = localhost

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
SSLEOF

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/server.key \
  -out certs/server.crt \
  -config certs/openssl.cnf \
  -extensions v3_req

# Extract CA cert for storage-cli config (both .crt and .pem for compatibility)
cp certs/server.crt certs/ca.crt
cp certs/server.crt certs/ca.pem

echo "2. Creating WebDAV server configuration..."
cat > config/httpd.conf <<'EOF'
ServerRoot "/usr/local/apache2"
Listen 443

LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule authn_file_module modules/mod_authn_file.so
LoadModule authn_core_module modules/mod_authn_core.so
LoadModule authz_host_module modules/mod_authz_host.so
LoadModule authz_user_module modules/mod_authz_user.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule auth_basic_module modules/mod_auth_basic.so
LoadModule dav_module modules/mod_dav.so
LoadModule dav_fs_module modules/mod_dav_fs.so
LoadModule setenvif_module modules/mod_setenvif.so
LoadModule ssl_module modules/mod_ssl.so
LoadModule unixd_module modules/mod_unixd.so
LoadModule dir_module modules/mod_dir.so

User daemon
Group daemon

# DAV Lock database
DAVLockDB /usr/local/apache2/var/DavLock

<IfModule ssl_module>
    SSLRandomSeed startup builtin
    SSLRandomSeed connect builtin
</IfModule>

<VirtualHost *:443>
    SSLEngine on
    SSLCertificateFile /usr/local/apache2/certs/server.crt
    SSLCertificateKeyFile /usr/local/apache2/certs/server.key

    DocumentRoot "/usr/local/apache2/webdav"

    <Directory "/usr/local/apache2/webdav">
        Dav On
        Options +Indexes
        AuthType Basic
        AuthName "WebDAV"
        AuthUserFile /usr/local/apache2/webdav.passwd
        Require valid-user

        <LimitExcept GET OPTIONS>
            Require valid-user
        </LimitExcept>
    </Directory>
</VirtualHost>
EOF

echo "3. Creating htpasswd file (user: testuser, password: testpass)..."
docker run --rm httpd:2.4 htpasswd -nb testuser testpass > config/webdav.passwd

echo "4. Creating docker-compose.yml..."
cat > docker-compose.yml <<'EOF'
version: '3.8'

services:
  webdav:
    image: httpd:2.4
    container_name: webdav-test
    ports:
      - "8443:443"
    volumes:
      - ./data:/usr/local/apache2/webdav
      - ./config/httpd.conf:/usr/local/apache2/conf/httpd.conf:ro
      - ./config/webdav.passwd:/usr/local/apache2/webdav.passwd:ro
      - ./certs:/usr/local/apache2/certs:ro
    restart: unless-stopped
EOF

echo "5. Starting WebDAV server..."
docker-compose up -d

echo "6. Setting proper permissions for WebDAV directory..."
sleep 2  # Wait for container to start
docker exec webdav-test mkdir -p /usr/local/apache2/var
docker exec webdav-test chmod 777 /usr/local/apache2/webdav
docker exec webdav-test chmod 777 /usr/local/apache2/var
docker exec webdav-test apachectl graceful  # Reload config

echo ""
echo "=== WebDAV Test Server Started ==="
echo "URL: https://localhost:8443"
echo "Username: testuser"
echo "Password: testpass"
echo ""
echo "To stop: cd webdav-test && docker-compose down"
echo ""
