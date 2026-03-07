FROM golang:1.25-bookworm

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH=/usr/local/go/bin:/usr/local/bin:${PATH}

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential \
      ca-certificates \
      file \
      git \
      libayatana-appindicator3-dev \
      libgtk-3-dev \
      libwebkit2gtk-4.1-dev \
      pkg-config \
      xz-utils && \
    rm -rf /var/lib/apt/lists/*

RUN GOBIN=/usr/local/bin /usr/local/go/bin/go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.40

WORKDIR /workspace
