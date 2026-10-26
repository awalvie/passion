# The release image. goreleaser builds the binary for each platform first, so
# this only copies it in. The Dockerfile at the root builds from source.
#
# There is no RUN step, so no platform needs emulation to build. alpine already
# has CA certificates, and the user needs no passwd entry.
FROM alpine:3

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/passion /usr/local/bin/passion

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/passion"]
