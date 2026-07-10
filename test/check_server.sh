#!/bin/bash
# 查看服务端采集器监控
HOST=${1:-localhost:8080}
curl -s "http://$HOST/api/collectors" | python3 -c "
import json,sys
d=json.load(sys.stdin)
if d:
    for c in d:
        print('ID:       '+c['Collector_ID'])
        print('名称:     '+c['Name'])
        print('来源:     '+c.get('Source_Host','-'))
        print('Kafka:    '+c['Address'])
        print('状态:     '+('启用' if c['Status']=='1' else '停用'))
        print('采集源:')
        for s in c['Source_Types']:
            print('  '+('V' if s['enabled'] else 'X')+' '+s['name'])
        print()
else:
    print('无采集器记录')
"
