FROM golang:alpine AS builder
WORKDIR /usr/src/app
COPY . .
RUN go build -o dstat main.go

FROM alpine AS runtime
WORKDIR /dstat
COPY --from=builder /usr/src/app/dstat .
ENTRYPOINT [ "/dstat/dstat" ]
