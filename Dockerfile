# ---- build stage --------------------------------------------------------
# Pin Go to the version recorded in go.mod (1.25). We build a fully static
# binary so the runtime stage can be distroless (no libc), which shrinks the
# image to ~25 MB and keeps the attack surface tiny.
FROM public.ecr.aws/docker/library/golang:1.25-alpine AS build

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS="-mod=readonly"

WORKDIR /src

# Cache module downloads in a dedicated layer so iterative builds skip the
# network whenever go.mod / go.sum have not changed.
COPY go.mod go.sum ./
RUN go mod download

# Pull the rest of the source. The internal/store/migrations files are
# go:embed'd, so they ship inside the binary - no separate COPY needed at
# runtime.
COPY . .

# Strip symbol tables (-s -w) so the image stays small. The version package
# reads a -X-injected commit string when set, but we keep the build flag
# optional so the default `docker build .` still works.
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN go build -trimpath \
        -ldflags="-s -w \
          -X github.com/cobanov/terminal-army-go/internal/version.Version=${VERSION} \
          -X github.com/cobanov/terminal-army-go/internal/version.Commit=${COMMIT} \
          -X github.com/cobanov/terminal-army-go/internal/version.Date=${DATE}" \
        -o /out/tarmy ./cmd/tarmy

# ---- runtime stage ------------------------------------------------------
# distroless/static is glibc-free, contains only CA certificates + tzdata,
# runs as non-root (uid 65532) by default, and has no shell. Perfect for a
# single static Go binary.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app
COPY --from=build /out/tarmy /usr/local/bin/tarmy

# The serve command listens on this port by default (TARMY_HTTP_ADDR).
EXPOSE 8080

# Sensible defaults; override via -e or compose environment.
ENV TARMY_HTTP_ADDR=0.0.0.0:8080 \
    TARMY_LOG_FORMAT=json \
    TARMY_LOG_LEVEL=info

ENTRYPOINT ["/usr/local/bin/tarmy"]
CMD ["serve"]
