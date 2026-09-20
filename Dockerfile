FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY server ./server
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /passion ./server/cmd/passion

FROM alpine:3

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 passion

COPY --from=build /passion /usr/local/bin/passion

USER passion
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/passion"]
