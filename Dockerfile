# The CLI image, built by GoReleaser from an already-compiled binary (hence no
# build stage — see .goreleaser.yaml's dockers_v2 section).
#
#   docker run --rm -e MISSU_TOKEN ghcr.io/glimpsed-io/missu status
FROM gcr.io/distroless/static-debian12:nonroot
COPY missu /missu
ENTRYPOINT ["/missu"]
