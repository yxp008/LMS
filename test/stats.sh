#!/bin/bash
# 查看服务端仪表盘统计（纯命令行）
HOST=${1:-localhost:8080}
echo ">>> 仪表盘"
curl -s "http://$HOST/api/stats" | python3 -m json.tool
echo ""
echo ">>> 等级分布"
curl -s "http://$HOST/api/levels" | python3 -m json.tool
echo ""
echo ">>> 来源分布"
curl -s "http://$HOST/api/sources" | python3 -m json.tool
