# LMS 日志管理系统

> V3.4 — 五层架构，Go 全栈（采集/处理/查询），Kafka 缓冲，三级存储

集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。

## 架构

```
┌── 采集层 ──┐    ┌── 处理层 ──┐    ┌──── 存储层 ────┐
│Vector+Go   │    │Go Processor│    │  ClickHouse    │
│NDJSON→Kafka│──►│脱敏+解析+写入│──►│ SSD→HDD→MinIO │
└────────────┘    └────────────┘    └───────┬────────┘
                                            │
                           ┌── 查询层 ──────┘
                           │日志API + 告警轮询
                           │server.go (Go) + goroutine
                           └───────┬────────┘
                                   │
                           ┌── 可视化层 ───┐
                           │  浏览器 SPA    │
                           │ 仪表盘/查询/告警│
                           └───────────────┘
```

五层同级，采集层/处理层/查询层优先使用 Go 以保证效率。

## 环境准备

### 1. 安装 ClickHouse

```bash
cd data/clickhouse_data
curl https://clickhouse.com/ | sh
```

### 2. 安装 Kafka

下载 Kafka 3.6 解压到 `~/kafka/`，配置 `config/kraft/server.properties` 中 `log.dirs` 指向持久目录，然后格式化存储：

```bash
~/kafka/bin/kafka-storage.sh format -t $(uuidgen) -c ~/kafka/config/kraft/server.properties
```

### 3. 安装 Vector

```bash
curl --proto '=https' -sSf https://sh.vector.dev | bash
```

### 4. 初始化数据库

```bash
# 启动 ClickHouse
cd data/clickhouse_data && ./clickhouse server --daemon

# 创建数据库和表
clickhouse-client --query "CREATE DATABASE IF NOT EXISTS LMS"
clickhouse-client --database LMS --multiquery < database_design/sql/LMS_Logs.sql
clickhouse-client --database LMS --multiquery < database_design/sql/LMS_Collectors.sql
clickhouse-client --database LMS --multiquery < database_design/sql/LMS_AlertRules.sql
clickhouse-client --database LMS --multiquery < database_design/sql/LMS_AlertTriggers.sql
```

> **注意**：`LMS_Logs` 表使用了 `storage_policy = 'hot_warm_cold'` 三级存储策略，需在 ClickHouse 配置中定义。`start.sh` 启动时自动从 `config_minimal.xml` 生成配置并替换路径占位符，无需手动配置。需提前创建温/冷存储目录：
> ```bash
> mkdir -p data/clickhouse_data/warm data/clickhouse_data/cold
> ```

### 一键安装

```bash
bash install.sh
```

## 快速开始

```bash
# 启动 / 停止
bash start.sh
bash stop.sh

# 访问
http://localhost:8080
```

## 安装

```bash
bash install.sh
```

脚本自动完成：Go 编译器安装、processor 和 reader 编译、默认配置生成。

**环境依赖（需手动准备）：**

| 依赖 | 路径 |
|---|---|
| Vector 0.56 | `~/.vector/bin/vector` |
| ClickHouse 26.6 | `data/clickhouse_data/clickhouse` |
| Kafka 3.6 | `~/kafka/` |
| Go 1.18+ | 脚本自动安装 |

**卸载：** `bash stop.sh` → `rm -rf LMS_mimo`

## 组件

| 组件 | 技术栈 | 说明 |
|---|---|---|
| 采集层 | Vector + Go | file 源读取 NDJSON，发往 Kafka |
| 消息队列 | Kafka 3.6 (KRaft) | 6 分区 zstd 压缩，topic: lms_elk_logs |
| 处理层 | Go | Kafka 消费 → 正则脱敏 → 字段解析 → 批量写入 |
| 存储层 | ClickHouse 26.6 | 列式 OLAP，三级存储：SSD(热)→HDD(温)→MinIO(冷) |
| 查询层 | Go | HTTP REST API（16 端点）+ 告警轮询 goroutine |
| 可视化层 | Vanilla JS + Chart.js | SPA，自定义日历组件 |

## ELK 日志采集流程

1. 将 NDJSON 文件放入 `collector/elk_logs/incoming/`
2. Vector 自动检测并发往 Kafka `lms_elk_logs`
3. Go Processor 消费 → 正则脱敏 → 字段映射 → 批量写入 ClickHouse
4. 前端来源选择「ELK本地日志文件」查看

JSON 数组格式需先用 Go reader 转换：
```bash
collector/elk_logs/reader input.json > collector/elk_logs/incoming/output.ndjson
```

## 脱敏规则

`processor/rules.json` 中配置，当前支持手机号和身份证号脱敏：

```
13812345678 → 138****5678
320106199001011234 → 320106199****11234
```

## 前端特性

- 自定义日历日期选择器（零依赖）
- 筛选器选择后自动触发查询
- 分页跳转输入框
- 日志级别动态加载，保留各源原始等级名称
- 来源/主机筛选
- 查询结果计数显示

## 目录结构

```
LMS_mimo/
├── collector/                    # 采集层（可独立部署）
│   ├── vector_wsl.toml.template  # Vector 配置模板
│   ├── collection_prefs.json     # 采集源开关
│   └── elk_logs/
│       ├── reader.go             # JSON→NDJSON 转换工具
│       └── incoming/             # 日志投放目录
├── processor/                    # 处理层（Go）
│   ├── main.go                   # Kafka→脱敏→ClickHouse
│   └── rules.json                # 脱敏规则
├── frontend/                     # 查询层 + 可视化层
│   ├── server.go                 # Go Web 服务 + 告警 (编译为 server)
│   ├── server.py                 # Python 原版（参考）
│   ├── index.html / app.js / style.css
├── data/clickhouse_data/         # ClickHouse
│   └── preprocessed_configs/
│       └── config_minimal.xml    # ClickHouse 配置（含存储策略）
├── database_design/
│   └── sql/                      # 建表 SQL（4 张表）
├── kafka/data/                   # Kafka 持久化
├── install.sh                    # 一键安装
├── start.sh / stop.sh            # 启停脚本
├── README.md
└── CLAUDE.md                     # 开发指南
```

## 移植

`bash install.sh && bash start.sh` 即可在新机器运行。所有路径基于 `${PROJECT_ROOT}` 自动计算，无需修改代码。

## 许可证

MIT
