FROM golang:1.26.7-alpine AS builder

RUN apk add --no-cache protobuf && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

WORKDIR /app

COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download

# Generate proto into gen/debt/v1 (within the backend module working dir)
COPY proto/ /proto/
RUN mkdir -p gen/debt/v1 && \
    protoc \
      --proto_path=/proto \
      --go_out=gen \
      --go_opt=paths=source_relative \
      --go-grpc_out=gen \
      --go-grpc_opt=paths=source_relative \
      /proto/debt/v1/debt.proto

COPY src/backend/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/server /bin/server

EXPOSE 50051

ENTRYPOINT ["/bin/server"]
