FROM golang:1.26.7-alpine AS builder

RUN apk add --no-cache protobuf && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

WORKDIR /app

COPY src/frontend/telegram/go.mod src/frontend/telegram/go.sum ./
RUN go mod download

# Generate proto into gen/debt/v1 (with frontend module path)
COPY proto/ /proto/
RUN mkdir -p gen/debt/v1 && \
    protoc \
      --proto_path=/proto \
      --go_out=gen \
      "--go_opt=Mdebt/v1/debt.proto=github.com/mrralexandrov/debt-bot/frontend/telegram/gen/debt/v1;debtv1" \
      --go_opt=paths=source_relative \
      --go-grpc_out=gen \
      "--go-grpc_opt=Mdebt/v1/debt.proto=github.com/mrralexandrov/debt-bot/frontend/telegram/gen/debt/v1;debtv1" \
      --go-grpc_opt=paths=source_relative \
      /proto/debt/v1/debt.proto

COPY src/frontend/telegram/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/bot ./cmd/bot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/bot /bin/bot

ENTRYPOINT ["/bin/bot"]
