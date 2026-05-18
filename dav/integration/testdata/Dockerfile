# Stage 1: Build ngx_dav_ext_module as a dynamic module
FROM nginx:1.28-alpine AS builder

RUN apk add --no-cache \
    build-base \
    pcre2-dev \
    openssl-dev \
    zlib-dev \
    libxml2-dev \
    libxslt-dev

ARG NGINX_VERSION=1.28.3
ARG DAV_EXT_VERSION=3.0.0

# Download nginx source and dav-ext module
RUN wget -qO- "https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz" | tar xz && \
    wget -qO- "https://github.com/arut/nginx-dav-ext-module/archive/refs/tags/v${DAV_EXT_VERSION}.tar.gz" | tar xz

# Build only the dynamic module with --with-compat for ABI compatibility
RUN cd nginx-${NGINX_VERSION} && \
    ./configure \
      --with-compat \
      --add-dynamic-module=../nginx-dav-ext-module-${DAV_EXT_VERSION} \
      --with-cc-opt='-Os -fstack-clash-protection -Wformat -Werror=format-security -g' \
      --with-ld-opt='-Wl,--as-needed,-O1,--sort-common' \
    && make modules

# Stage 2: Runtime image based on official nginx
FROM nginx:1.28-alpine

# Install runtime dependencies and utilities
RUN apk add --no-cache \
    libxml2 \
    libxslt \
    apache2-utils

# Copy the compiled dav-ext module from builder
COPY --from=builder \
     /nginx-1.28.3/objs/ngx_http_dav_ext_module.so \
     /usr/lib/nginx/modules/ngx_http_dav_ext_module.so

# Create required directories
RUN mkdir -p /var/www/webdav /etc/nginx/certs \
    && chown -R nginx:nginx /var/www/webdav \
    && chmod 755 /var/www/webdav

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf
COPY htpasswd /etc/nginx/htpasswd
COPY certs/server.crt /etc/nginx/certs/server.crt
COPY certs/server.key /etc/nginx/certs/server.key

# Ensure correct permissions
RUN chmod 644 /etc/nginx/htpasswd /etc/nginx/certs/server.crt \
    && chmod 600 /etc/nginx/certs/server.key

EXPOSE 443

CMD ["nginx", "-g", "daemon off;"]
