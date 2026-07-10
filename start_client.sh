#!/bin/bash
# LMS 客户端启动脚本 — 仅采集器 (Vector + 管理界面)
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export LMS_PROJECT_ROOT="$SCRIPT_DIR"

# 加载配置文件
[ -f "$SCRIPT_DIR/config.env" ] && source "$SCRIPT_DIR/config.env"

COLLECTOR_PORT=${COLLECTOR_PORT:-8081}

mkdir -p "$SCRIPT_DIR/logs"

echo "========================================"
echo "  LMS 客户端（采集器）"
echo "========================================"

# 检查服务端连通性
SERVER_URL=${LMS_SERVER_URL:-http://localhost:8080}
echo "  服务端地址: $SERVER_URL"
echo "  Kafka:      ${LMS_KAFKA_BROKER:-localhost:9092}"
echo ""

# 启动采集器管理服务（自动管理 Vector）
if pgrep -f "server.*-collector" > /dev/null 2>&1; then
    echo "采集器服务已在运行"
else
    nohup $SCRIPT_DIR/frontend/server -collector > "$SCRIPT_DIR/logs/collector.log" 2>&1 &
    disown
    sleep 3
    if pgrep -f "server.*-collector" > /dev/null 2>&1; then
        echo "采集器服务启动成功: http://localhost:$COLLECTOR_PORT"
    else
        echo "采集器服务启动失败，请检查 logs/collector.log"
        exit 1
    fi
fi

# 确认 Vector
sleep 1
if pgrep -f "vector --config" > /dev/null 2>&1; then
    echo "Vector 已启动"
else
    echo "Vector 未运行（注册采集器并启用采集源后将自动启动）"
fi

echo ""
echo "========================================"
echo "  客户端已启动"
echo "  管理界面: http://localhost:$COLLECTOR_PORT"
echo ""
echo "  使用步骤:"
echo "    1. 打开管理界面"
echo "    2. 注册采集器 → 设置 Kafka 地址 → 勾选采集源"
echo "    3. 保存后自动开始采集并传输至服务端"
echo "========================================"
