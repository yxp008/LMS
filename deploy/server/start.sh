#!/bin/bash
# LMS 服务端部署启动脚本
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
export LMS_PROJECT_ROOT="$PROJECT_ROOT"

echo "========================================"
echo "  LMS 日志管理服务端"
echo "========================================"

# 1. ClickHouse
if curl -s "http://localhost:8123/?query=SELECT%201" > /dev/null 2>&1; then
    echo "ClickHouse 已在运行"
else
    $PROJECT_ROOT/data/clickhouse_data/clickhouse server --daemon 2>/dev/null
    sleep 5
    echo "ClickHouse 已启动"
fi

# 2. Kafka
if pgrep -f "kafka.Kafka" > /dev/null 2>&1; then
    echo "Kafka 已在运行"
else
    if [ -f "$HOME/kafka/bin/kafka-server-start.sh" ]; then
        $HOME/kafka/bin/kafka-server-start.sh -daemon $HOME/kafka/config/kraft/server.properties 2>/dev/null
        sleep 8
        echo "Kafka 已启动"
    else
        echo "Kafka 未安装，跳过"
    fi
fi

# 3. Processor
if pgrep -f "processor/processor" > /dev/null 2>&1; then
    echo "Processor 已在运行"
else
    nohup $PROJECT_ROOT/processor/processor > $PROJECT_ROOT/logs/processor.log 2>&1 &
    sleep 2
    echo "Processor 已启动"
fi

# 4. 服务端 Web
if curl -s "http://localhost:8080/api/stats" > /dev/null 2>&1; then
    echo "服务端 Web 已在运行"
else
    nohup $PROJECT_ROOT/server > $PROJECT_ROOT/logs/frontend.log 2>&1 &
    sleep 3
    echo "服务端 Web 已启动: http://localhost:8080"
fi

echo ""
echo "========================================"
echo "  服务端: http://localhost:8080"
echo "  ClickHouse: http://localhost:8123"
echo "  Kafka: localhost:9092"
echo "========================================"
