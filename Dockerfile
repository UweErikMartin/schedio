# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25.6
ARG OUTFILE=schedio

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build-stage

ARG OUTFILE=schedio
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/${OUTFILE:-schedio} ./cmd/${OUTFILE:-schedio}

FROM --platform=$TARGETPLATFORM alpine AS runtime
ARG OUTFILE=schedio
RUN apk add --no-cache libcap ca-certificates

WORKDIR /app
COPY --from=build-stage /out/${OUTFILE} /app/schedio

# Allow binding privileged ports (<1024) while running as non-root.
RUN setcap cap_net_bind_service=+ep /app/schedio

FROM scratch
COPY --from=runtime /app /app
ENV PORT=80
EXPOSE 80
USER 65532:65532
ENTRYPOINT ["/app/schedio"]
