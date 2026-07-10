# LMS 测试指南

> 所有测试命令均可在纯终端（无浏览器）环境下执行。输出为 JSON 格式，通过 `python3 -m json.tool` 美化显示。

## 客户端（采集器）

| 命令 | 含义 |
|---|---|
| `curl -s http://localhost:8081/api/collectors \| python3 -m json.tool` | 查看已注册的采集器 |
| `curl -s -X POST http://localhost:8081/api/collectors -H 'Content-Type: application/json' -d '{"action":"register","Name":"MyCollector","Address":"服务端IP:9092"}' \| python3 -m json.tool` | 注册一个新采集器 |
| `curl -s -X POST http://localhost:8081/api/collection-prefs -H 'Content-Type: application/json' -d '{"linux_system_logs":true,"network_device_logs":true,"elk_file_logs":true,"kafka_broker":"服务端IP:9092"}' \| python3 -m json.tool` | 启用采集源并设置 Kafka 地址 |
| `curl -s -X POST http://localhost:8081/api/collection-prefs -H 'Content-Type: application/json' -d '{"test_kafka":"服务端IP:9092"}' \| python3 -m json.tool` | 测试 Kafka 连通性 |
| `curl -s -X POST http://localhost:8081/api/collection-prefs -H 'Content-Type: application/json' -d '{"kafka_broker":"新地址","collector_name":"新名称"}' \| python3 -m json.tool` | 修改 Kafka 地址和采集器名称 |

## 服务端

| 命令 | 含义 |
|---|---|
| `curl -s http://localhost:8080/api/stats \| python3 -m json.tool` | 仪表盘统计（总量/错误/警告/24h） |
| `curl -s http://localhost:8080/api/levels \| python3 -m json.tool` | 按日志级别分布 |
| `curl -s http://localhost:8080/api/timeline?hours=24 \| python3 -m json.tool` | 近24小时时间线 |
| `curl -s http://localhost:8080/api/sources \| python3 -m json.tool` | 按来源类型分布 |
| `curl -s http://localhost:8080/api/hosts \| python3 -m json.tool` | 按主机分布（前20） |
| `curl -s 'http://localhost:8080/api/logs?page=1&page_size=20' \| python3 -m json.tool` | 分页查询日志 |
| `curl -s 'http://localhost:8080/api/logs?page=1&page_size=10&level=3' \| python3 -m json.tool` | 查询最近10条错误 |
| `curl -s 'http://localhost:8080/api/logs?search=关键词' \| python3 -m json.tool` | 搜索日志内容 |
| `curl -s http://localhost:8080/api/collectors \| python3 -m json.tool` | 查看所有已注册采集器 |
| `curl -s -X POST http://localhost:8080/api/collectors -H 'Content-Type: application/json' -d '{"action":"delete","Collector_ID":"C_xxx"}' \| python3 -m json.tool` | 删除采集器 |
| `curl -s http://localhost:8080/api/alert-rules \| python3 -m json.tool` | 查看告警规则 |
| `curl -s http://localhost:8080/api/alert-triggers \| python3 -m json.tool` | 查看告警触发记录 |

## 快捷脚本

```bash
bash test/check_client.sh                    # 查看本机客户端采集器
bash test/check_client.sh 192.168.1.101:8081 # 查看远程客户端
bash test/check_server.sh                    # 查看本机服务端
bash test/check_server.sh 192.168.1.100:8080 # 查看远程服务端
bash test/register.sh                        # 一键注册采集器
bash test/register.sh 客户端:8081 Kafka:9092 采集器名
bash test/stats.sh                           # 仪表盘概览
```

## 日志查询参数

| 参数 | 说明 | 示例 |
|---|---|---|
| `page` | 页码 | `page=1` |
| `page_size` | 每页条数 | `page_size=50` |
| `level` | 级别过滤 | `level=3`（ERROR） |
| `host` | 主机过滤 | `host=192.168.1.1` |
| `source_type` | 来源过滤 | `source_type=Linux系统日志` |
| `search` | 内容搜索 | `search=error` |
| `start_time` / `end_time` | 日期范围 | `start_time=2026-07-01` |
| `hours` | 最近N小时 | `hours=24` |
