#!/bin/bash
# LMS 日志管理系统 - 启动脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
CH_BIN="$PROJECT_ROOT/data/clickhouse_data/clickhouse"
CH_CFG="$PROJECT_ROOT/data/clickhouse_data/preprocessed_configs/config.xml"
CH_CFG_SAFE="$PROJECT_ROOT/data/clickhouse_data/preprocessed_configs/config_minimal.xml"
CH_HTTP="http://localhost:8123"
VECTOR_BIN="$HOME/.vector/bin/vector"
VECTOR_CFG="$PROJECT_ROOT/collector/vector_wsl.toml"
KAFKA_BIN="$HOME/kafka/bin/kafka-server-start.sh"
KAFKA_CFG="$HOME/kafka/config/kraft/server.properties"
PROCESSOR_BIN="$PROJECT_ROOT/processor/processor"
FRONTEND_PORT=8080

echo "=========================================="
echo "  LMS 日志管理系统 启动脚本"
echo "=========================================="

# 1. 启动 ClickHouse
echo ""
echo "[1/6] 启动 ClickHouse..."
if curl -s "$CH_HTTP/?query=SELECT%201" > /dev/null 2>&1; then
    echo "  -> ClickHouse 已在运行"
else
    # 预处理配置经常损坏，先用最小可用配置覆盖
    if [ -f "$CH_CFG_SAFE" ]; then
        cp "$CH_CFG_SAFE" "$CH_CFG"
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
echo "[2/6] 启动 Kafka..."
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

# 3. 启动 Vector 采集器
echo ""
echo "[3/6] 启动 Vector 采集器..."
if pgrep -f "vector --config" > /dev/null 2>&1; then
    echo "  -> Vector 已在运行"
else
    nohup $VECTOR_BIN --config $VECTOR_CFG > /tmp/vector.log 2>&1 &
    disown
    sleep 3
    if pgrep -f "vector --config" > /dev/null 2>&1; then
        echo "  -> Vector 启动成功"
    else
        echo "  -> Vector 启动失败，请检查 /tmp/vector.log"
        exit 1
    fi
fi

# 4. 启动 Processor (Kafka → 脱敏 → ClickHouse)
echo ""
echo "[4/6] 启动 Processor..."
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

# 5. 启动前端服务
echo ""
echo "[5/6] 启动前端服务..."
if curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
    echo "  -> 前端服务已在运行"
else
    # 释放可能被残留进程占用的端口
    fuser -k $FRONTEND_PORT/tcp 2>/dev/null
    sleep 1
    nohup $PROJECT_ROOT/frontend/server > /tmp/lms_frontend.log 2>&1 &
    disown
    # 前端启动较慢（需初始化 Vector），等最多 10 秒
    for i in 1 2 3 4 5; do
        sleep 2
        if curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
            echo "  -> 前端服务启动成功"
            break
        fi
    done
    if ! curl -s "http://localhost:$FRONTEND_PORT/api/stats" > /dev/null 2>&1; then
        echo "  -> 前端服务启动失败"
        exit 1
    fi
fi

# 6. 启动告警检查器
echo ""
echo "[6/6] 启动告警检查器..."
if pgrep -f "alert_checker.py" > /dev/null 2>&1; then
    echo "  -> 告警检查器已在运行"
else
    nohup python3 $PROJECT_ROOT/frontend/alert_checker.py > /tmp/lms_alert_checker.log 2>&1 &
    disown
    sleep 2
    if pgrep -f "alert_checker.py" > /dev/null 2>&1; then
        echo "  -> 告警检查器启动成功"
    else
        echo "  -> 告警检查器启动失败"
    fi
fi

echo ""
echo "=========================================="
echo "  系统已启动!"
echo "  前端界面: http://localhost:$FRONTEND_PORT"
echo "  ClickHouse: $CH_HTTP"
echo "  Kafka: localhost:9092"
echo "  Vector: syslog端口 1514 (UDP/TCP)"
echo "  告警检查器: 后台运行中"
echo "=========================================="
