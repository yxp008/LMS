#!/bin/bash
# 注册采集器 + 启用采集源 + 测试 Kafka（纯命令行）
CLIENT=${1:-localhost:8081}
BROKER=${2:-localhost:9092}
NAME=${3:-Vector-WSL}

echo ">>> 注册采集器"
curl -s -X POST "http://$CLIENT/api/collectors" \
  -H 'Content-Type: application/json' \
  -d "{\"action\":\"register\",\"Name\":\"$NAME\",\"Address\":\"$BROKER\"}" | python3 -m json.tool

echo ""
echo ">>> 启用采集源"
curl -s -X POST "http://$CLIENT/api/collection-prefs" \
  -H 'Content-Type: application/json' \
  -d "{\"linux_system_logs\":true,\"network_device_logs\":true,\"elk_file_logs\":true,\"kafka_broker\":\"$BROKER\",\"collector_name\":\"$NAME\"}" | python3 -m json.tool

echo ""
echo ">>> 测试 Kafka 连通性"
curl -s -X POST "http://$CLIENT/api/collection-prefs" \
  -H 'Content-Type: application/json' \
  -d "{\"test_kafka\":\"$BROKER\"}" | python3 -m json.tool

echo ""
echo ">>> 查看结果"
curl -s "http://$CLIENT/api/collectors" | python3 -m json.tool
