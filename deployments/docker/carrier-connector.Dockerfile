FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY apps/carrier-connector/go.mod apps/carrier-connector/go.sum ./
RUN go mod download

COPY apps/carrier-connector/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -o carrier-connector .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/carrier-connector .

CMD ["./carrier-connector"]
