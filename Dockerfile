FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata xorriso
COPY proxclt /usr/local/bin/proxclt
ENTRYPOINT ["proxclt"]
CMD ["--help"]
