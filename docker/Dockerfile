FROM golang:1.19.4-bullseye as Builder
WORKDIR /app
RUN apt-get update && apt-get install -y upx
ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOPROXY=https://goproxy.cn
COPY . ./
RUN go mod download
RUN go build -o rssc
RUN upx -9 -o rssc.minify rssc && mv rssc.minify rssc

FROM debian:bullseye
COPY --from=Builder /app/rssc /bin/rssc
COPY --from=Builder /app/rules /rules
ENV TZ=Asia/Shanghai
RUN cp /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone
EXPOSE 8080
CMD ["rssc"]