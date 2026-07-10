#!/bin/bash
# LMS 日志管理系统 - 启动脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
export LMS_PROJECT_ROOT="$PROJECT_ROOT"

# 加载配置文件
[ -f "$PROJECT_ROOT/config_server.env" ] && source "$PROJECT_ROOT/config_server.env"

SERVER_PORT=${SERVER_PORT:-8080}
COLLECTOR_PORT=${COLLECTOR_PORT:-8081}
CLICKHOUSE_PORT=${CLICKHOUSE_PORT:-8123}
KAFKA_PORT=${KAFKA_PORT:-9092}
KAFKA_HOME=${KAFKA_HOME:-$HOME/kafka}

CH_BIN="$PROJECT_ROOT/data/clickhouse_data/clickhouse"
CH_CFG="$PROJECT_ROOT/data/clickhouse_data/preprocessed_configs/config.xml"
CH_CFG_SAFE="$PROJECT_ROOT/data/clickhouse_data/preprocessed_configs/config_minimal.xml"
CH_HTTP="http://localhost:$CLICKHOUSE_PORT"
VECTOR_BIN="$HOME/.vector/bin/vector"
VECTOR_CFG="$PROJECT_ROOT/collector/vector_wsl.toml"
KAFKA_BIN="$KAFKA_HOME/bin/kafka-server-start.sh"
KAFKA_CFG="$KAFKA_HOME/config/kraft/server.properties"
PROCESSOR_BIN="$PROJECT_ROOT/processor/processor"
FRONTEND_PORT=$SERVER_PORT

echo "=========================================="
echo "  LMS 日志管理系统 启动脚本"
echo "=========================================="

# 1. 启动 ClickHouse
echo ""
echo "[1/5] 启动 ClickHouse..."
if curl -s "$CH_HTTP/?query=SELECT%201" > /dev/null 2>&1; then
    echo "  -> ClickHouse 已在运行"
else
    # 预处理配置经常损坏，先用最小可用配置覆盖并替换路径
    if [ -f "$CH_CFG_SAFE" ]; then
        sed "s|__PROJECT_ROOT__|$PROJECT_ROOT|g" "$CH_CFG_SAFE" > "$CH_CFG"
    fi
    $CH_BIN server --config-file=$CH_CFG --daemon 2>/dev/null
    # ClickHouse 启动较慢，最多等 15 秒
    for i in 1 2 3 4 5; do
        sleep 3
        if curl -s "$CH_HTTP/?query=SELECT%201" > /dev/null 2>&1; then
            echo "  -> ClickHouse 启动成功"
            break
        fi
    done
    if ! curl -s "$CH_HTTP/?query=SELECT%201" > /dev/null 2>&1; then
        echo "  -> ClickHouse 启动失败，请检查日志"
        exit 1
    fi
fi

# 2. 启动 Kafka
echo ""
echo "[2/5] 启动 Kafka..."
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

# 3. 启动 Processor (Kafka → 脱敏 → ClickHouse)
echo ""
echo "[3/5] 启动 Processor..."
if pgrep -f "processor/processor" > /dev/null 2>&1; then
    echo "  -> Processor 已在运行"
else
    nohup $PROCESSOR_BIN > /tmp/processor.log 2>&1 &
    disown
    sleep 2
    if pgrep -f "processor/processor" > /dev/null 2>&1; then
        echo "  -> Processor 启动成功"
    else
        echo "  -> Processor 启动失败，请检查 /tmp/processor.log"
    fi
fi

# 4. 启动前端服务（Go server 自动生成 Vector 配置并启动 Vector + 告警）
echo ""
echo "[4/5] 启动前端服务..."
if curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
    echo "  -> 前端服务已在运行"
else
    fuser -k $FRONTEND_PORT/tcp 2>/dev/null
    sleep 1
    nohup $PROJECT_ROOT/frontend/server > $PROJECT_ROOT/logs/frontend.log 2>&1 &
    disown
    for i in 1 2 3 4 5; do
        sleep 2
        if curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
            echo "  -> 服务端启动成功（日志管理 :8080）"
            break
        fi
    done
    if ! curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
        echo "  -> 服务端启动失败"
        exit 1
    fi
fi

# 独立采集器管理服务
echo ""
echo "  启动采集器管理服务（:8081）..."
if curl -s "http://localhost:8081/api/collection-prefs" > /dev/null 2>&1; then
    echo "  -> 采集器管理已在运行"
else
    nohup $PROJECT_ROOT/frontend/server -collector > $PROJECT_ROOT/logs/collector.log 2>&1 &
    disown
    sleep 2
    if curl -s "http://localhost:8081/api/collection-prefs" > /dev/null 2>&1; then
        echo "  -> 采集器管理启动成功（:8081）"
    else
        echo "  -> 采集器管理启动失败"
    fi
fi

# 5. 确认 Vector 已由采集器服务启动
echo ""
echo "[5/5] 确认 Vector..."
sleep 2
if pgrep -f "vector --config" > /dev/null 2>&1; then
    echo "  -> Vector 已由前端服务启动"
else
    echo "  -> Vector 未运行，可能是所有采集源已禁用"
fi

echo ""
echo "=========================================="
echo "  系统已启动!"
echo "  前端界面: http://localhost:$FRONTEND_PORT"
echo "  服务端 (日志管理):  http://localhost:8080"
echo "  采集器 (管理):      http://localhost:8081"
echo "  ClickHouse:         $CH_HTTP"
echo "  Kafka:              localhost:$KAFKA_PORT"
echo "=========================================="
