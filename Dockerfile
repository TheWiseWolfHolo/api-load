FROM --platform=$BUILDPLATFORM node:20-alpine AS builder

ARG VERSION=1.0.0
WORKDIR /build
COPY ./web/package*.json ./
RUN npm ci
COPY ./web .
RUN VITE_VERSION=${VERSION} npm run build


FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS builder2

ARG VERSION=1.0.0
ARG TARGETOS
ARG TARGETARCH
ENV GO111MODULE=on \
    CGO_ENABLED=0

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-s -w -X api-load/internal/version.Version=${VERSION}" -o api-load


FROM alpine

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates

COPY --from=builder2 /build/api-load .
EXPOSE 3001
ENTRYPOINT ["/app/api-load"]
