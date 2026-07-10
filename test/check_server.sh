#!/bin/bash
# 查看服务端采集器监控（纯命令行）
HOST=${1:-localhost:8080}
curl -s "http://$HOST/api/collectors" | python3 -m json.tool
