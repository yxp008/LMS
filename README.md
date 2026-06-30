# LMS 日志管理系统

集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。

## 架构

```
采集层（客户端）          消息队列           处理层（服务端）
┌──────────────┐       ┌──────────┐       ┌──────────────┐
│ ELK NDJSON   │──►    │          │   ──►│ Go Processor  │
│ Vector(file) │──►    │  Kafka   │   ──►│ 脱敏+解析+入库 │──► ClickHouse
└──────────────┘       └──────────┘       └──────────────┘
                                                 │
                                          server.py(:8080)
                                                 │
                                          浏览器 SPA 前端
```

## 快速开始

```bash
# 启动全部服务
bash start.sh

# 停止全部服务
bash stop.sh

# 访问前端
http://localhost:8080
```

## 组件

| 组件 | 技术栈 | 说明 |
|---|---|---|
| 采集层 | Vector + Go | file 源读取 NDJSON，发往 Kafka |
| 消息队列 | Kafka 3.6 (KRaft) | 6 分区 zstd 压缩，topic: lms_elk_logs |
| 处理层 | Go | Kafka 消费 → 正则脱敏 → 批量写 ClickHouse |
| 存储 | ClickHouse 26.6 | 列式 OLAP，Asia/Shanghai 时区 |
| 后端 | Python stdlib | REST API + 静态文件服务 |
| 前端 | Vanilla JS + Chart.js | SPA 单页应用，自定义日历组件 |
| 告警 | Python | 5 秒轮询，邮件/Webhook 通知 |

## ELK 日志采集

1. 将 NDJSON 文件放入 `collector/elk_logs/incoming/`
2. Vector 自动检测并发送至 Kafka
3. Processor 脱敏后写入 ClickHouse
4. 前端日志查询选择来源「ELK本地日志文件」

JSON 数组格式需先用 Go reader 转换：`collector/elk_logs/reader input.json > output.ndjson`

## 目录结构

```
LMS_mimo/
├── collector/              # 采集层
│   ├── vector_wsl.toml.template
│   ├── collection_prefs.json
│   └── elk_logs/incoming/  # 日志投放目录
├── processor/              # 处理层（Go）
│   ├── main.go
│   ├── rules.json          # 脱敏规则
│   └── processor
├── frontend/               # Web 前后端
│   ├── server.py
│   ├── alert_checker.py
│   ├── index.html / app.js / style.css
├── data/clickhouse_data/   # ClickHouse
├── kafka/data/             # Kafka 持久化
├── start.sh / stop.sh      # 启停脚本
└── CLAUDE.md               # 开发指南
```

## 许可证

MIT
