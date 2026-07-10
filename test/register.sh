#!/bin/bash
# 注册采集器（无浏览器时使用）
# 用法: bash test/register.sh [客户端地址] [服务端IP:9092]
CLIENT=${1:-localhost:8081}
BROKER=${2:-localhost:9092}
NAME=${3:-Vector-WSL}

echo "注册采集器..."
echo "  客户端: $CLIENT"
echo "  Kafka:  $BROKER"
echo "  名称:   $NAME"
echo ""

# 1. 注册
R=$(curl -s -X POST "http://$CLIENT/api/collectors" \
  -H 'Content-Type: application/json' \
  -d "{\"action\":\"register\",\"Name\":\"$NAME\",\"Address\":\"$BROKER\"}")
echo "注册: $R"

CID=$(echo "$R" | python3 -c "import json,sys; print(json.load(sys.stdin).get('Collector_ID',''))" 2>/dev/null)

# 2. 启用采集源
R2=$(curl -s -X POST "http://$CLIENT/api/collection-prefs" \
  -H 'Content-Type: application/json' \
  -d "{\"linux_system_logs\":true,\"network_device_logs\":true,\"elk_file_logs\":true,\"kafka_broker\":\"$BROKER\",\"collector_name\":\"$NAME\"}")
echo "采集源: $R2"

# 3. 测试 Kafka
R3=$(curl -s -X POST "http://$CLIENT/api/collection-prefs" \
  -H 'Content-Type: application/json' \
  -d "{\"test_kafka\":\"$BROKER\"}")
echo "Kafka:  $R3"

echo ""
echo "完成。查看: bash test/check_client.sh"
