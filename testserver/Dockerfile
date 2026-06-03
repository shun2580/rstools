FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /testserver-bin ./testserver/

FROM alpine:3.21
COPY --from=builder /testserver-bin /testserver-bin
EXPOSE 8443
CMD ["/testserver-bin"]
