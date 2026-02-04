ARG BASE_IMAGE=gcr.io/distroless/static-debian12:nonroot

# Build stage
FROM golang:1.25-alpine AS builder

# Git commit passed from build machine (avoids installing git in container)
ARG GIT_COMMIT=unknown

# Install build dependencies
RUN apk add --no-cache make

WORKDIR /build

# Copy source code
COPY . .

# Tidy and verify Go module dependencies
RUN go mod tidy && go mod verify

# Build binary using make to include version, commit, and build date
RUN make build GIT_COMMIT=${GIT_COMMIT}

# Runtime stage
FROM ${BASE_IMAGE}

WORKDIR /app

# Copy binary from builder (make build outputs to bin/)
COPY --from=builder /build/bin/hyperfleet-adapter /app/adapter

<<<<<<< HEAD
=======
# Config files are NOT packaged in the image - they must come from ConfigMaps
# Mount the adapter config via ConfigMap at deployment time:
#   volumeMounts:
#   - name: config
#     mountPath: /etc/adapter/config
#   volumes:
#   - name: config
#     configMap:
#       name: adapter-config
#
# Set ADAPTER_CONFIG_PATH environment variable to point to the mounted config:
#   env:
#   - name: ADAPTER_CONFIG_PATH
#     value: /etc/adapter/adapterconfig.yaml

>>>>>>> 1e51a34 (fix: Moved version to a package version and fixed maestro integration running failure)
ENTRYPOINT ["/app/adapter"]
CMD ["serve"]

LABEL name="hyperfleet-adapter" \
      vendor="Red Hat" \
      version="0.1.0" \
      summary="HyperFleet Adapter - Event-driven adapter services for HyperFleet cluster provisioning" \
      description="Handles CloudEvents consumption, AdapterConfig CRD integration, precondition evaluation, Kubernetes Job creation/monitoring, and status reporting via API"
