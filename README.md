# LMS 日志管理系统

> V5.1 — 五层架构，Go 全栈，Kafka 缓冲，三级存储，客户端/服务端分离，交互式AI对话

集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。

## 功能

| 模块 | 说明 |
|---|---|
| 仪表盘 | 日志总量/错误/警告统计，级别分布图，来源分布图 |
| 日志查询 | 分页查询，按级别/主机/来源/日期/关键词过滤，自动刷新 |
| 可视化分析 | 时间线图、主机排名、来源占比，等级过滤 |
| AI 分析 | 基于 DeepSeek 的交互式对话，自动分析日志数据 |
| 采集器管理 | 注册/编辑采集器，管理采集源开关，Kafka 连通性测试 |
| 告警规则 | 自定义 SQL 告警，邮件/SMS/Webhook 通知，定时轮询 |
| 日志脱敏 | 正则规则脱敏（手机号、身份证等），processor/rules.json 可配置 |
| 三级存储 | SSD(0-7d) → HDD(7-30d) → MinIO(30-180d) 自动迁移 |

## 架构

```
客户端 (机器A)                           服务端 (机器B)
┌────────────────────┐                 ┌────────────────────────────┐
│ client/            │                 │ server/                    │
│ ├── collector/     │──Kafka:9092──►  │ ├── processor/             │
│ │   Vector + reader│                 │ │   Kafka→脱敏→ClickHouse   │
│ └── ...            │──注册:8080──►   │ ├── data/clickhouse_data/  │
│                    │                 │ ├── kafka/                 │
│ frontend/(共用)    │                 │ ├── database_design/       │
│ Go server          │                 │ └── ...                    │
│   -collector :8081 │                 │                            │
└────────────────────┘                 │ frontend/(共用)            │
                                       │ Go server :8080            │
                                       └────────────────────────────┘
```

## 前置依赖

| 组件 | 安装方式 |
|---|---|
| Go | `sudo apt-get install -y golang-go` |
| Vector | `curl --proto '=https' --tlsv1.2 -sSf https://sh.vector.dev \| bash`（安装到 `~/.vector/`） |
| Kafka | 下载 [kafka_2.13-3.6.0.tgz](https://archive.apache.org/dist/kafka/3.6.0/) 解压到 `~/kafka/` |
| ClickHouse | 下载 [clickhouse](https://packages.clickhouse.com/tgz/stable/) 二进制放入 `data/clickhouse_data/` |

> 仅客户端部署不需要 Kafka 和 ClickHouse。

## 安装

```bash
git clone <repo-url>
cd LMS
bash server/install.sh && bash client/install.sh
```

安装脚本 自动完成：检查 Go → 编译 processor/server/reader → 检查外部依赖 → 初始化配置文件。

按角色安装：

```bash
# 仅客户端
git clone --filter=blob:none --sparse <repo-url> && cd LMS
git sparse-checkout set client/ frontend/ test/ install.sh README.md
bash client/install.sh

# 仅服务端
git clone --filter=blob:none --sparse <repo-url> && cd LMS
git sparse-checkout set server/ frontend/ test/ install.sh README.md
bash server/install.sh
```

## 启动

```bash
# 服务端
bash server/start_server.sh

# 客户端
bash client/start_client.sh

# 仅服务端
bash server/start_server.sh

# 仅客户端
bash client/start_client.sh
```

访问：
- 服务端（日志管理）：`http://localhost:8080`
- 客户端（采集器管理）：`http://localhost:8081`

## 分离部署

客户端 `config_client.env` 修改两个地址指向服务端：

```bash
LMS_KAFKA_BROKER=<服务端IP>:9092
LMS_SERVER_URL=http://<服务端IP>:8080
```

客户端启动后，打开 `http://<客户端IP>:8081` 注册采集器，服务端 `:8080` 即可监控到。

## 目录结构

```
LMS/
├── client/                          # 客户端
│   ├── collector/                   # Vector 配置 + ELK reader
│   ├── config_client.env            # 客户端配置
│   ├── install.sh / start_client.sh
│   └── test/
├── server/                          # 服务端
│   ├── processor/                   # Kafka → ClickHouse
│   ├── database_design/sql/         # 建表 SQL
│   ├── config_server.env
│   ├── install.sh / start_server.sh
│   └── test/
├── frontend/                        # 共用（Go Web 服务 + 前端 SPA）
│   ├── server.go                    # Go 服务（-collector 切换模式）
│   ├── collector_state.go           # 采集器本地状态
│   ├── index.html / app.js / style.css
│   └── server                       # 编译产物
├── data/clickhouse_data/            # ClickHouse 运行时（服务端）
├── kafka/                           # Kafka 运行时（服务端）
├── test/                            # 测试脚本 + API 参考
├── stop.sh
└── README.md / CLAUDE.md
```

## 测试

```bash
bash test/check_client.sh            # 查看客户端采集器
bash test/check_server.sh            # 查看服务端采集器
bash test/register.sh                # 注册采集器
bash test/stats.sh                   # 仪表盘统计
```

所有 API 端点详见 `test/Test.md`。

## 许可证

MIT
