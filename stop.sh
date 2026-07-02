#!/bin/bash
# LMS 日志管理系统 - 停止脚本

echo "=========================================="
echo "  LMS 日志管理系统 停止脚本"
echo "=========================================="

# 1. 停止 Vector 采集器
echo ""
echo "[1/6] 停止 Vector..."
pkill -f "vector --config" 2>/dev/null
sleep 1
if pgrep -f "vector --config" > /dev/null 2>&1; then
    pkill -9 -f "vector"
    sleep 1
fi
echo "  -> Vector 已停止"

# 2. 停止 Processor
echo ""
echo "[2/6] 停止 Processor..."
pkill -f "processor/processor" 2>/dev/null
sleep 1
echo "  -> Processor 已停止"

# 3. 停止 Kafka
echo ""
echo "[3/6] 停止 Kafka..."
pkill -f "kafka.Kafka" 2>/dev/null
sleep 2
if pgrep -f "kafka.Kafka" > /dev/null 2>&1; then
    pkill -9 -f "kafka.Kafka"
    sleep 2
fi
echo "  -> Kafka 已停止"

# 4. 停止前端服务
echo ""
echo "[4/6] 停止前端服务..."
pkill -f "frontend/server" 2>/dev/null
fuser -k 8080/tcp 2>/dev/null
sleep 1
echo "  -> 前端服务已停止"

# 4. 停止告警检查器
echo ""
echo "[5/6] 停止告警检查器..."
pkill -f "alert_checker.py" 2>/dev/null
sleep 1
echo "  -> 告警检查器已停止"

# 5. 停止 ClickHouse
echo ""
echo "[6/6] 停止 ClickHouse..."
pkill -f "clickhouse server" 2>/dev/null
pkill -f "clickhouse-watchdog" 2>/dev/null
sleep 3
if pgrep -f "clickhouse" > /dev/null 2>&1; then
    pkill -9 -f "clickhouse"
    sleep 2
fi
fuser -k 8123/tcp 2>/dev/null
echo "  -> ClickHouse 已停止"

echo ""
echo "=========================================="
echo "  所有服务已停止"
echo "=========================================="
