#!/bin/bash
# 查看服务端仪表盘统计
HOST=${1:-localhost:8080}
curl -s "http://$HOST/api/stats" | python3 -c "
import json,sys
s=json.load(sys.stdin)
print('日志总量:   '+str(s.get('total_logs',0)))
print('错误日志:   '+str(s.get('error_count',0)))
print('警告日志:   '+str(s.get('warn_count',0)))
print('近24小时:   '+str(s.get('last_24h',0)))
"
