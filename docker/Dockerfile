FROM denoland/deno:bin-2.9.8 AS Deno
FROM golang:1.27.3-bookworm AS Builder
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ARG VERSION=dev
ARG COMMIT=HEAD
ARG BUILD_DATE=undefined
RUN go build -trimpath -ldflags="-s -w -X github.com/soulteary/RSS-Can/internal/version.Version=${VERSION} -X github.com/soulteary/RSS-Can/internal/version.Commit=${COMMIT} -X github.com/soulteary/RSS-Can/internal/version.BuildDate=${BUILD_DATE}" -o /rssc .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata libstdc++6 && \
    rm -rf /var/lib/apt/lists/*
COPY --from=Deno /deno /usr/local/bin/deno
COPY --from=Builder /rssc /bin/rssc
COPY --from=Builder /app/rules /rules
COPY --from=Builder /app/LICENSE /app/CREDITS.md /usr/share/licenses/rss-can/
COPY --from=Builder /app/third_party/licenses /usr/share/licenses/rss-can/dependencies
COPY --from=Builder /app/internal/jssdk/js/*.LICENSE /usr/share/licenses/rss-can/embedded/
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo Asia/Shanghai > /etc/timezone
ARG VERSION=dev
ARG COMMIT=HEAD
ARG BUILD_DATE=undefined
LABEL org.opencontainers.image.title="RSS Can" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/soulteary/rss-can"
EXPOSE 8080
CMD ["rssc"]
