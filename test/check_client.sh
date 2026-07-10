#!/bin/bash
# 查看客户端采集器状态（纯命令行）
HOST=${1:-localhost:8081}
curl -s "http://$HOST/api/collectors" | python3 -m json.tool
