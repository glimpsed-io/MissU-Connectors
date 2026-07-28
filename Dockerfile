# The CLI image, built by GoReleaser from an already-compiled binary (hence no
# build stage — see .goreleaser.yaml's dockers_v2 section).
#
#   docker run --rm -e MISSU_TOKEN ghcr.io/glimpsed-io/miss-u-more status
FROM gcr.io/distroless/static-debian12:nonroot
COPY miss-u-more /miss-u-more
ENTRYPOINT ["/miss-u-more"]
