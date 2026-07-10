#!/bin/bash
# LMS 服务端启动脚本 — Kafka + ClickHouse + Processor + Web 服务
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export LMS_PROJECT_ROOT="$SCRIPT_DIR"

# 加载服务端配置
[ -f "$SCRIPT_DIR/server/config_server.env" ] && source "$SCRIPT_DIR/config_server.env"

SERVER_PORT=${SERVER_PORT:-8080}
KAFKA_HOME=${KAFKA_HOME:-$HOME/kafka}

CH_BIN="$SCRIPT_DIR/data/clickhouse_data/clickhouse"
CH_CFG="$SCRIPT_DIR/data/clickhouse_data/preprocessed_configs/config.xml"
CH_CFG_SAFE="$SCRIPT_DIR/data/clickhouse_data/preprocessed_configs/config_minimal.xml"
CH_URL="${LMS_CLICKHOUSE_URL:-http://localhost:8123}"
KAFKA_BIN="$KAFKA_HOME/bin/kafka-server-start.sh"
KAFKA_CFG="$KAFKA_HOME/config/kraft/server.properties"
PROCESSOR_BIN="$SCRIPT_DIR/processor/processor"

mkdir -p "$SCRIPT_DIR/logs"

echo "========================================"
echo "  LMS 服务端"
echo "========================================"

# 1. ClickHouse
echo ""
echo "[1/4] 启动 ClickHouse..."
if curl -s "$CH_URL/?query=SELECT%201" > /dev/null 2>&1; then
    echo "  -> ClickHouse 已在运行"
else
    if [ -f "$CH_CFG_SAFE" ]; then
        sed "s|__PROJECT_ROOT__|$SCRIPT_DIR|g" "$CH_CFG_SAFE" > "$CH_CFG"
    fi
    $CH_BIN server --config-file=$CH_CFG --daemon 2>/dev/null
    for i in 1 2 3 4 5; do
        sleep 3
        if curl -s "$CH_URL/?query=SELECT%201" > /dev/null 2>&1; then
            echo "  -> ClickHouse 启动成功"
            break
        fi
    done
    if ! curl -s "$CH_URL/?query=SELECT%201" > /dev/null 2>&1; then
        echo "  -> ClickHouse 启动失败"
        exit 1
    fi
fi

# 2. Kafka
echo ""
echo "[2/4] 启动 Kafka..."
if pgrep -f "kafka.Kafka" > /dev/null 2>&1; then
    echo "  -> Kafka 已在运行"
else
    $KAFKA_BIN -daemon $KAFKA_CFG 2>/dev/null
    for i in 1 2 3 4 5; do
        sleep 2
        if pgrep -f "kafka.Kafka" > /dev/null 2>&1; then
            echo "  -> Kafka 启动成功"
            break
        fi
    done
    if ! pgrep -f "kafka.Kafka" > /dev/null 2>&1; then
        echo "  -> Kafka 启动失败"
        exit 1
    fi
fi

# 3. Processor
echo ""
echo "[3/4] 启动 Processor..."
if pgrep -f "processor/processor" > /dev/null 2>&1; then
    echo "  -> Processor 已在运行"
else
    nohup $PROCESSOR_BIN > "$SCRIPT_DIR/../logs/processor.log" 2>&1 &
    disown
    sleep 2
    if pgrep -f "processor/processor" > /dev/null 2>&1; then
        echo "  -> Processor 启动成功"
    else
        echo "  -> Processor 启动失败，请检查 logs/processor.log"
    fi
fi

# 4. Web 服务 (server 模式)
echo ""
echo "[4/4] 启动 Web 服务..."
if curl -s "http://localhost:$SERVER_PORT/api/stats" > /dev/null 2>&1; then
    echo "  -> Web 服务已在运行"
else
    fuser -k $SERVER_PORT/tcp 2>/dev/null
    sleep 1
    nohup $SCRIPT_DIR/../frontend/server > "$SCRIPT_DIR/../logs/frontend.log" 2>&1 &
    disown
    for i in 1 2 3 4 5; do
        sleep 2
        if curl -s "http://localhost:$SERVER_PORT/api/stats" > /dev/null 2>&1; then
            echo "  -> 服务端启动成功"
            break
        fi
    done
    if ! curl -s "http://localhost:$SERVER_PORT/api/stats" > /dev/null 2>&1; then
        echo "  -> 服务端启动失败，请检查 logs/frontend.log"
        exit 1
    fi
fi

echo ""
echo "========================================"
echo "  服务端已启动"
echo "  前端界面: http://localhost:$SERVER_PORT (日志管理)"
echo "========================================"
