# Stage 1: install system prerequisites, Go toolchain & Go CI tools
FROM ubuntu:22.04 AS builder

# 1. System packages
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      curl git ca-certificates build-essential \
 && rm -rf /var/lib/apt/lists/*

# 2. Install Go 1.24.3 (latest 1.24 patch)
ENV GO_VERSION=1.24.3
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    | tar -C /usr/local -xz

ENV GOPATH=/go \
    PATH=/usr/local/go/bin:/go/bin:$PATH

# 3. Install Go CI tools into $GOPATH/bin
RUN CGO_ENABLED=0 go install honnef.co/go/tools/cmd/staticcheck@latest \
 && CGO_ENABLED=0 go install github.com/securego/gosec/v2/cmd/gosec@latest \
 && CGO_ENABLED=0 go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ==================================================
# Stage 2: install Node.js and frontend dependencies
FROM ubuntu:22.04 AS frontend-builder

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# 1. Node 18
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash - \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y nodejs \
 && rm -rf /var/lib/apt/lists/*

# 2. Prep workspace and install frontend deps **in the GitHub Actions mount path**
WORKDIR /github/workspace/services/frontend
COPY services/frontend/package.json services/frontend/package-lock.json ./
RUN npm ci --legacy-peer-deps

# ==================================================
# Final image: combine both toolchains
FROM ubuntu:22.04

# 1. System deps for runtime
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      ca-certificates git curl build-essential \
 && rm -rf /var/lib/apt/lists/*

# 2. Copy Go toolchain + tools from builder
COPY --from=builder /usr/local/go /usr/local/go
COPY --from=builder /go /go

# 3. Install Node.js directly in the final image
RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash - \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

# Copy pre-built frontend dependencies (from frontend-builder stage)
COPY --from=frontend-builder /github/workspace/services/frontend \
                               /github/workspace/services/frontend

# 4. Set environment
ENV GOPATH=/go \
    PATH=/usr/local/go/bin:/go/bin:/usr/bin:$PATH

# 5. Default workdir
WORKDIR /github/workspace

# 6. Entrypoint
ENTRYPOINT ["/bin/bash", "-lc"]
