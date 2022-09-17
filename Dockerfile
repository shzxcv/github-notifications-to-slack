FROM golang:1.19.1-alpine3.16@sha256:ca4f0513119dfbdc65ae7b76b69688f0723ed00d9ecf9de68abbf6ed01ef11bf as builder
WORKDIR /builder
COPY *.go .
ENV CGO_ENABLED 0
ENV GOOS linux
RUN go build -o slack-notifier-actions main.go && chmod +x ./slack-notifier-actions

FROM alpine:3.16@sha256:1304f174557314a7ed9eddb4eab12fed12cb0cd9809e4c28f29af86979a3c870
WORKDIR /slack
COPY --from=builder /builder/slack-notifier-actions .
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
