FROM node:22-alpine AS client

WORKDIR /src
RUN corepack enable && corepack prepare pnpm@12.3.4 --activate

COPY client/package.json client/pnpm-lock.yaml ./client/
RUN cd client && pnpm install --frozen-lockfile

COPY client ./client
# Vite writes to ../server/web/dist, which the go stage copies out.
RUN cd client && pnpm build

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY server ./server
COPY --from=client /src/server/web/dist ./server/web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /passion ./server/cmd/passion

FROM alpine:3

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 passion

COPY --from=build /passion /usr/local/bin/passion

USER passion
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/passion"]
