## syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
WORKDIR /app
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
	go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/imap-spam-cleaner .

FROM node:24-alpine
ARG CLAUDE_CODE_VERSION=2.1.302
RUN npm install --global "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
	&& npm cache clean --force
WORKDIR /app
COPY --from=build /out/imap-spam-cleaner /app/imap-spam-cleaner
RUN touch store.db
EXPOSE 8080
ENTRYPOINT [ "/app/imap-spam-cleaner" ]
