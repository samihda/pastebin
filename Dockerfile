FROM golang:1.24.2-alpine3.21 AS builder

WORKDIR /app
COPY . /app

RUN CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -o pastebin ./

FROM scratch

ENV PORT=8000
EXPOSE $PORT

COPY --from=builder /app/pastebin .
COPY --from=builder /app/static ./static

CMD [ "./pastebin" ]
