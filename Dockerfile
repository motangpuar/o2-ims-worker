FROM alpine:3.20.3

LABEL maintainer "ninoagus@protonmail.com"

# Install the necessary packages
RUN apk add --no-cache \
    bash \
    unzip \
    dnsmasq \
    ca-certificates \
    gcompat \
    wget \
    make \
    gcc \
    musl-dev \
    git

# Install Go 1.24.0 manually
RUN wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz && \
    rm go1.24.0.linux-amd64.tar.gz

# Set Go environment
ENV PATH=/usr/local/go/bin:$PATH
ENV GOPATH=/go
ENV CGO_ENABLED=1


ENV MEMTEST_VERSION 5.31b
ENV SYSLINUX_VERSION 6.03
ENV TEMP_SYSLINUX_PATH /tmp/syslinux-"$SYSLINUX_VERSION"

WORKDIR /tmp/ims-worker
COPY . .
RUN make build
RUN cp build/ims-worker /usr/bin/ims-worker


WORKDIR /tmp
RUN \
  mkdir -p "$TEMP_SYSLINUX_PATH" \
  && wget -q https://www.kernel.org/pub/linux/utils/boot/syslinux/syslinux-"$SYSLINUX_VERSION".tar.gz \
  && tar -xzf syslinux-"$SYSLINUX_VERSION".tar.gz \
  && mkdir -p /var/lib/tftpboot \
  && cp "$TEMP_SYSLINUX_PATH"/bios/core/pxelinux.0 /var/lib/tftpboot/ \
  && cp "$TEMP_SYSLINUX_PATH"/bios/com32/libutil/libutil.c32 /var/lib/tftpboot/ \
  && cp "$TEMP_SYSLINUX_PATH"/bios/com32/elflink/ldlinux/ldlinux.c32 /var/lib/tftpboot/ \
  && cp "$TEMP_SYSLINUX_PATH"/bios/com32/menu/menu.c32 /var/lib/tftpboot/ \
  && rm -rf "$TEMP_SYSLINUX_PATH" \
  && rm /tmp/syslinux-"$SYSLINUX_VERSION".tar.gz \
  && wget -q http://www.memtest.org/download/archives/"$MEMTEST_VERSION"/memtest86+-"$MEMTEST_VERSION".bin.gz \
  && gzip -d memtest86+-"$MEMTEST_VERSION".bin.gz \
  && mkdir -p /var/lib/tftpboot/memtest \
  && mv memtest86+-$MEMTEST_VERSION.bin /var/lib/tftpboot/memtest/memtest86+

#RUN mkdir -p /var/lib/tftpboot/pxelinuc.cfg/
#COPY pxelinux.cfg/ /var/lib/tftpboot/pxelinux.cfg/

#COPY build/ims-worker /usr/bin/ims-worker
