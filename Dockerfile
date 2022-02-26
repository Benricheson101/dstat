FROM golang:alpine AS builder
WORKDIR /usr/src/app
COPY . .
RUN go build -o dstat main.go

FROM alpine AS runtime
COPY --from=builder /usr/src/app/dstat /bin
ENTRYPOINT [ "dstat" ]
