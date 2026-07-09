# LMS 日志管理系统

> V4.1 — 五层架构，Go 全栈，Kafka 缓冲，三级存储，客户端/服务端分离，AI分析

集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。

## 架构

```
客户端 (电脑A)                       服务端 (电脑B)
┌──────────────────┐               ┌──────────────────────────┐
│ 采集层           │               │        采集器监控(只读)    │
│ Vector(file)     │──Kafka:9092──►│ 处理层                   │
│ Go server        │               │ Processor(Go)            │
│   -collector     │──注册:8080──►│         ↓                │
│     :8081        │               │ 存储层                   │
│                  │               │ ClickHouse(SSD→HDD→MinIO)│
└──────────────────┘               │         ↓                │
                                   │ 查询层                   │
                                   │ Go server(:8080)+告警     │
                                   │         ↓                │
                                   │ 可视化层                 │
                                   │ 浏览器 SPA               │
                                   └──────────────────────────┘
```

## 快速开始

```bash
# 首次安装
bash install.sh

# 修改 config.env 自定义端口（可选）

# 启动
bash start.sh

# 访问
服务端: http://localhost:8080
采集器: http://localhost:8081
```

## 分离部署

**客户端（采集器）**：`bash deploy/client/start.sh`
- 配置传输地址指向服务端 Kafka `服务端IP:9092`
- 设环境变量 `LMS_SERVER_URL=http://服务端IP:8080` 自动注册

**服务端（日志管理）**：`bash deploy/server/start.sh`
- 需要 ClickHouse + Kafka + Processor

详见 `deploy/README.md`

## 配置

所有端口通过 `config.env` 修改：

```bash
SERVER_PORT=8080        # 服务端
COLLECTOR_PORT=8081     # 采集器
CLICKHOUSE_PORT=8123
KAFKA_PORT=9092
KAFKA_HOME=$HOME/kafka
SYSLOG_PORT=1514
```

## 组件

| 层级 | 技术栈 | 说明 |
|---|---|---|
| 采集层 | Vector + Go | file 源读取 NDJSON，发往 Kafka |
| 消息队列 | Kafka 3.6 (KRaft) | 6 分区 zstd 压缩 |
| 处理层 | Go Processor | Kafka → 脱敏 → 解析 → ClickHouse |
| 存储层 | ClickHouse 26.6 | SSD(0-7d)→HDD(7-30d)→MinIO(30-180d) |
| 查询层 | Go server | REST API + 告警 goroutine |
| 可视化层 | Vanilla JS | SPA，自定义日历，零 CDN 依赖 |

## ELK 日志采集

1. 将 NDJSON 文件放入 `collector/elk_logs/incoming/`
2. Vector 自动检测 → Kafka → Processor → ClickHouse
3. 前端来源选择「ELK本地日志文件」

JSON 数组转换：`collector/elk_logs/reader input.json > output.ndjson`

## 目录结构

```
LMS/
├── collector/              # 采集层
│   ├── vector_wsl.toml.template
│   ├── elk_logs/incoming/  # 日志投放
│   └── collector_state.json # 客户端本地状态
├── processor/              # 处理层 (Go)
├── frontend/               # 查询层 + 可视化层
│   ├── server.go           # Go 服务 (-collector 切换模式)
│   └── index.html / app.js / style.css
├── data/clickhouse_data/   # 存储层
├── kafka/                  # 消息队列
├── deploy/                 # 分离部署脚本
│   ├── client/start.sh
│   └── server/start.sh
├── config.env              # 端口配置
├── install.sh / start.sh / stop.sh
└── README.md
```

## 许可证

MIT
