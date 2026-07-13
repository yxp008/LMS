# LMS 多阶段构建 — 编译 Go 二进制
FROM golang:1.23-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY . .

# 编译 Processor
WORKDIR /build/server/processor
RUN go mod tidy && go build -o processor main.go

# 编译 Server (含 collector 模式)
WORKDIR /build/frontend
RUN go build -o server server.go collector_state.go

# ===== 运行时镜像 =====
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl bash procps && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
WORKDIR /app

# 复制编译产物
COPY --from=builder /build/server/processor/processor /app/processor/
COPY --from=builder /build/frontend/server /app/frontend/
# 复制配置和静态文件
COPY --from=builder /build/server/processor/rules.json /app/processor/
COPY --from=builder /build/frontend/index.html /app/frontend/
COPY --from=builder /build/frontend/app.js /app/frontend/
COPY --from=builder /build/frontend/style.css /app/frontend/
COPY --from=builder /build/client/collector/vector_wsl.toml.template /app/client/collector/
COPY --from=builder /build/server/database_design/sql/ /app/server/database_design/sql/
COPY --from=builder /build/docker/entrypoint.sh /app/

# 创建运行时目录
RUN mkdir -p /app/logs /app/client/collector/vector_data /app/client/collector/elk_logs/incoming

ENV LMS_PROJECT_ROOT=/app
ENV TZ=Asia/Shanghai

EXPOSE 8080 8081
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["server"]
