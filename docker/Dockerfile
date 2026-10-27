FROM golang:1.27.3-bookworm AS Builder
WORKDIR /app
ENV CGO_ENABLED=1
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -trimpath -ldflags="-s -w" -o /rssc .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata libstdc++6 && \
    rm -rf /var/lib/apt/lists/*
COPY --from=Builder /rssc /bin/rssc
COPY --from=Builder /app/rules /rules
ENV TZ=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo Asia/Shanghai > /etc/timezone
EXPOSE 8080
CMD ["rssc"]
