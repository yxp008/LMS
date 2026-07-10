#!/bin/bash
# LMS 服务端安装脚本 — 编译服务端组件
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "========================================"
echo "  LMS 服务端安装"
echo "========================================"

# 1. Go
echo ""
echo "[1/4] 检查 Go..."
if command -v go &> /dev/null; then
    echo "  -> Go $(go version | awk '{print $3}') 已安装"
else
    echo "  -> 未找到 Go，正在安装..."
    sudo apt-get update -qq && sudo apt-get install -y -qq golang-go
    echo "  -> Go 安装完成"
fi

# 2. 编译 processor
echo ""
echo "[2/4] 编译 Processor（Kafka → 脱敏 → ClickHouse）..."
cd "$PROJECT_ROOT/server/processor"
GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o processor . 2>/dev/null || {
    GOPROXY=https://goproxy.cn,direct go build -o processor .
}
echo "  -> processor 编译成功"

# 3. 编译 server
echo ""
echo "[3/4] 编译 Server（Web 服务 + 告警）..."
cd "$PROJECT_ROOT/frontend"
GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o server server.go collector_state.go 2>/dev/null || {
    GOPROXY=https://goproxy.cn,direct go build -o server server.go collector_state.go
}
echo "  -> server 编译成功"

# 4. 检查外部依赖
echo ""
echo "[4/4] 检查外部依赖..."
CH_PATH="$PROJECT_ROOT/server/data/clickhouse_data/clickhouse"
if [ -f "$CH_PATH" ]; then
    echo "  -> ClickHouse: $CH_PATH ✓"
else
    echo "  -> ClickHouse: 未找到（请将 clickhouse 二进制放入 server/data/clickhouse_data/）"
fi
KAFKA_PATH="$HOME/kafka/bin/kafka-server-start.sh"
if [ -f "$KAFKA_PATH" ]; then
    echo "  -> Kafka: $HOME/kafka/ ✓"
else
    echo "  -> Kafka: 未找到（请下载解压到 ~/kafka/）"
fi

mkdir -p "$PROJECT_ROOT/logs"

echo ""
echo "========================================"
echo "  服务端安装完成"
echo ""
echo "  启动: bash server/start_server.sh"
echo "========================================"
