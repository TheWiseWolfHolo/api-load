FROM node:20-alpine AS frontend-builder

WORKDIR /build
COPY ./web .
RUN npm install
RUN npm run build


FROM golang:1.24-alpine AS backend-builder

WORKDIR /build
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /build/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o gpt-load


FROM alpine:latest

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
COPY --from=backend-builder /build/gpt-load .

EXPOSE 3000
ENTRYPOINT ["/app/gpt-load"]
