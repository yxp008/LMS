#!/bin/bash
# LMS Docker 入口 — 根据参数启动服务端或客户端
set -e

export LMS_PROJECT_ROOT=/app
export LMS_CLICKHOUSE_URL="${LMS_CLICKHOUSE_URL:-http://clickhouse:8123}"
export LMS_KAFKA_BROKER="${LMS_KAFKA_BROKER:-kafka:9092}"
export LMS_SERVER_URL="${LMS_SERVER_URL:-http://server:8080}"

case "${1:-server}" in
  server)
    echo ">>> 启动 LMS 服务端 (:8080)"
    exec /app/frontend/server
    ;;
  collector)
    echo ">>> 启动 LMS 采集器 (:8081)"
    exec /app/frontend/server -collector
    ;;
  processor)
    echo ">>> 启动 LMS Processor"
    exec /app/server/processor/processor
    ;;
  *)
    echo "用法: docker run ... [server|collector|processor]"
    exit 1
    ;;
esac
