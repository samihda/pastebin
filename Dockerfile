FROM --platform=$BUILDPLATFORM golang:1.24.2-alpine3.21 AS builder

WORKDIR /app
COPY . /app

ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -o pastebin ./

FROM scratch

ENV PORT=8000
EXPOSE $PORT

COPY --from=builder /app/pastebin .

CMD [ "./pastebin" ]
