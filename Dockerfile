FROM golang:1.19.1-alpine3.16@sha256:ca4f0513119dfbdc65ae7b76b69688f0723ed00d9ecf9de68abbf6ed01ef11bf as builder
WORKDIR /builder
COPY . .
ENV CGO_ENABLED 0
ENV GOOS linux
RUN go build -o github-notifications-to-slack main.go && chmod +x ./github-notifications-to-slack

FROM alpine:3.16@sha256:1304f174557314a7ed9eddb4eab12fed12cb0cd9809e4c28f29af86979a3c870
COPY --from=builder /builder/github-notifications-to-slack /usr/bin/github-notifications-to-slack
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
