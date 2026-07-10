# CLAUDE.md

> **V4.2 版本** — 五层架构，Go 全栈（采集/处理/查询），Kafka 缓冲，三级存储，日志等级保留原始值，支持网络设备日志采集，可移植部署，全文件上传，客户端/服务端分离，环境变量配置跨机器部署，交互式AI对话。

本文件为 Claude Code (claude.ai/code) 在此仓库中工作时提供指导。

## 项目概述

LMS（日志管理系统）— 集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。DIY 架构：无框架、无构建工具、无包管理器。

## 架构

六个独立进程，由 `start.sh` / `stop.sh` 编排：

```
┌─────────── 采集层 ───────────┐
│ Vector(file) + Go reader     │
│ NDJSON → Kafka               │
└──────────────┬───────────────┘
               │
┌─────────── 处理层 ───────────┐
│ Go Processor                 │
│ 脱敏 + 字段解析 + 批量写入    │
└──────────────┬───────────────┘
               │
┌─────────── 存储层 ───────────┐
│ ClickHouse (列式 OLAP)       │
│ 三级存储: SSD→HDD→MinIO      │
└──────────────┬───────────────┘
               │
┌─────────── 查询层 ───────────┐
│ 日志查询 API + 告警轮询       │
│ server.go (Go) + goroutine   │
└──────────────┬───────────────┘
               │
┌─────────── 可视化层 ─────────┐
│ 浏览器 SPA (Vanilla JS)      │
│ 仪表盘 / 日志查询 / 告警管理   │
└──────────────────────────────┘
```

五层架构，同级协作：

- **采集层**（`collector/`）：可独立部署在客户端。Vector file 源 + Go reader 读取 NDJSON / JSON 数组文件，发往 Kafka topic `lms_elk_logs`。支持 journald 和 syslog 源（当前关闭）。**技术栈：Vector + Go。**
- **Kafka**（v3.6.0，KRaft 模式）：采集层与处理层之间的消息队列缓冲。Topic `lms_elk_logs` 6 分区 zstd 压缩，数据目录 `kafka/data/`。
- **处理层**（`processor/`）：Go 编译的独立程序，消费 Kafka → 正则脱敏 → 解析 ELK JSON 字段 → 批量写入 ClickHouse。**技术栈：Go。**
- **存储层**（`data/clickhouse_data/`）：ClickHouse v26.6.1，列式 OLAP 数据库。时区 `Asia/Shanghai`。四张表：`LMS.LMS_Logs`、`LMS.LMS_Collectors`、`LMS.LMS_AlertRules`、`LMS.LMS_AlertTriggers`。**三级存储策略 (hot_warm_cold)**：热(0-7d SSD)→温(7-30d HDD)→冷(30-180d MinIO对象存储)。配置见 `config_minimal.xml`。
- **查询层**（`frontend/server.go`）：Go 编译的独立二进制。HTTP REST API（16 个端点）+ 告警规则定时轮询（goroutine，每 5 秒）二合一。原 Python 文件保留作参考。
- **可视化层**（`frontend/`）：原生 JS SPA（index.html + app.js + style.css），Chart.js 4.4.4 CDN 加载，自定义日历日期选择器。**技术栈：Vanilla JS。**
## 关键路径

| 路径 | 用途 |
|---|---|
| `frontend/server.go` | Go Web 服务 + 告警 goroutine（已编译为 `frontend/server`） |
| `frontend/server.py` | Python 原版（保留作参考） |
| `frontend/alert_checker.py` | Python 原告警（参考） |
| `frontend/server` | Go 编译的查询层二进制 |
| `frontend/app.js` | SPA 全部逻辑（原生 JS） |
| `frontend/style.css` | 全部样式（含自定义日期选择器） |
| `collector/vector_wsl.toml` | 生成的 Vector 采集配置（**勿直接编辑**） |
| `collector/vector_wsl.toml.template` | 模板文件，含 `# ===== COLLECTION: xxx =====` 标记 |
| `collector/collection_prefs.json` | 持久化的采集源开关 + ELK 文件路径 |
| `collector/elk_logs/incoming/` | ELK NDJSON 文件投放目录 |
| `collector/elk_logs/reader` | Go 编译的 JSON 数组 → NDJSON 转换工具 |
| `processor/main.go` | Go 处理程序源码（Kafka 消费 → 脱敏 → ClickHouse） |
| `processor/rules.json` | 脱敏正则规则配置 |
| `frontend/collector_state.go` | 采集器本地状态（ID/名称/地址/来源） |
| `collector/collector_state.json` | 采集器运行时状态持久化文件 |
| `collector/collection_prefs.json` | 采集源开关 + Kafka broker 持久化 |
| `config.env` | 统一配置文件（所有端口+地址） |

### Collector_ID 生成规则

- **生成时机**：客户端首次注册采集器时（非启动时）
- **格式**：`C_<前6位hostname>_<8位随机hex>`，如 `C_LAPTOP_7d8ce52f`
- **不可变性**：编辑采集器配置不会重新生成 ID
- **Source_Host**：注册时自动获取 `hostname (IP)` 格式的来源地址，同步至服务端

### AI 分析

- 独立子标签页，与仪表盘/日志查询/可视化分析并列
- 基于 DeepSeek API 的交互式对话
- 前端 `localStorage` 存储 API Key，调用 `api.deepseek.com`
- Markdown 格式回复（通过 marked.js 渲染）
- 「分析数据」按钮：收集图表数据后自动分析
- 不支持 AI 功能的采集器端不显示 AI 子标签


| `processor/processor` | Go 编译的处理器二进制 |
| `kafka/data/` | Kafka 持久化数据目录 |
| `data/clickhouse_data/` | ClickHouse 二进制、数据和配置 |
| `data/clickhouse_data/preprocessed_configs/config_minimal.xml` | ClickHouse 最小可用配置备份 |

## 跨机器部署环境变量

所有地址通过环境变量配置，支持客户端/服务端分离部署：

| 变量 | 默认值 | 用途 |
|---|---|---|
| `LMS_PROJECT_ROOT` | 自动检测 | 项目根目录 |
| `LMS_CLICKHOUSE_URL` | `http://localhost:8123` | ClickHouse HTTP 地址（处理器+服务端） |
| `LMS_KAFKA_BROKER` | `localhost:9092` | Kafka broker 地址（处理器+采集器） |
| `LMS_KAFKA_TOPIC` | `lms_elk_logs` | Kafka topic 名 |
| `LMS_KAFKA_GROUP` | `lms-processor` | Kafka 消费者组 |
| `LMS_SERVER_URL` | `http://localhost:8080` | 服务端地址（采集器注册目标） |
| `LMS_VECTOR_BIN` | `$HOME/.vector/bin/vector` | Vector 二进制路径 |
| `SERVER_PORT` | 8080 | 服务端 Web 端口 |
| `COLLECTOR_PORT` | 8081 | 采集器管理端口 |

**分离部署时**，在客户端机器的 `config.env` 中设置：
```bash
LMS_CLICKHOUSE_URL=http://<服务端IP>:8123
LMS_KAFKA_BROKER=<服务端IP>:9092
LMS_SERVER_URL=http://<服务端IP>:8080
```

## 常用命令

```bash
# 启动 / 停止全部服务（6 个组件）
bash start.sh
bash stop.sh

# 单独启动组件
frontend/server
# 告警已合并到 server 中 (goroutine)
./processor/processor &

# 直接查询 ClickHouse
curl -s 'http://localhost:8123/' -d 'SELECT 1'

# 健康检查
curl http://localhost:8080/api/stats
pgrep -f "vector --config"
pgrep -f "processor/processor"
pgrep -f "frontend/server"

# 查看日志
tail -f /tmp/vector.log
tail -f /tmp/processor.log
tail -f /tmp/lms_frontend.log

# 重新生成 Vector 配置
# Vector 配置通过 API 触发：curl -X POST http://localhost:8080/api/collection-prefs ...

# 通过 API 启用/禁用采集类型
curl -X POST http://localhost:8080/api/collection-prefs -H 'Content-Type: application/json' -d '{"elk_file_logs": true}'

# 强制重启前端
fuser -k 8080/tcp; sleep 2; nohup $PROJECT_ROOT/frontend/server > /tmp/lms_frontend.log 2>&1 & disown

# 重新采集 ELK 数据（清除进度 + 删除 ClickHouse 数据）
rm -rf collector/vector_data/elk_file
curl -s 'http://localhost:8123/' -d "ALTER TABLE LMS.LMS_Logs DELETE WHERE Source_Type='ELK本地日志文件'"
pkill -x vector; sleep 2; nohup ~/.vector/bin/vector --config collector/vector_wsl.toml > /tmp/vector.log 2>&1 & disown

# JSON 数组 → NDJSON 转换
./collector/elk_logs/reader /path/to/array.json > /path/to/output.ndjson

# 查看 Kafka topic 状态
~/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --describe --topic lms_elk_logs
```

## API 端点

全部位于 `http://localhost:8080`：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/stats` | 仪表盘统计概览 |
| GET | `/api/logs?page=&page_size=&level=&host=&source_type=&search=&start_time=&end_time=` | 分页日志查询（无去重，原始条目） |
| GET | `/api/levels` | 按日志级别统计 |
| GET | `/api/timeline?hours=24` | 时间序列统计 |
| GET | `/api/sources` | 按来源类型统计（始终包含全部已知类型） |
| GET | `/api/hosts` | 按主机统计（前 20） |
| GET | `/api/collectors` | 查询采集器列表（含采集源元数据） |
| POST | `/api/collectors` | 更新采集器状态 |
| GET/POST | `/api/collection-prefs` | 读取/写入采集开关。POST 触发 Vector 配置重新生成和重启 |
| GET/POST | `/api/alert-rules` | 查询 / 增删改告警规则 |
| GET/POST | `/api/smtp-config` | 读取/写入 SMTP 配置 |
| POST | `/api/smtp-test` | 测试 SMTP 连接 |
| POST | `/api/query` | 执行任意 SQL |
| POST | `/api/sql-validate` | 验证 SQL 语法 |
| POST | `/api/webhook` | 入站 Webhook 接收 |
| GET | `/api/alert-triggers` | 告警规则触发统计 |

## 数据流详解

### ELK 日志采集全链路

```
1. export_data.ndjson 放入 collector/elk_logs/incoming/
2. Vector file 源检测到新 .ndjson，逐行读取
3. Kafka sink 以 text 编码发送到 lms_elk_logs（仅发原始行，不带 Vector 元数据）
4. Processor(Go) 从 Kafka 消费每条消息
5. 正则脱敏（rules.json 中的规则作用于 message 和 syslog_message 字段）
6. 字段映射：host.ip → Host，syslog_message → Message，@timestamp → Timestamp
7. 其余字段存入 Tags（JSON）
8. Log_ID = L + 毫秒时间戳 + 4位随机数，每条日志唯一
9. 批量写入 ClickHouse（200,000 条/批）
```

### 脱敏规则

`processor/rules.json` 中配置正则规则。手机号中间四位隐藏示例：

```
原始：13812345678 → 脱敏后：138****5678
正则：(1[3-9]\d)\d{4}(\d{4}) → $1****$2
```

## Vector 配置模板系统

`collector/vector_wsl.toml.template` 使用注释标记按采集类型控制配置段的显隐：

```
# ===== COLLECTION: linux_system_logs =====
[sources.journald]
...
# ===== END COLLECTION =====

# ===== INPUTS =====
inputs = ["cleanup_journald", "cleanup_syslog"]
# ===== END INPUTS =====
```

Go server 启动时调用 `generateVectorConfig(prefs)` 读取模板，根据 `collection_prefs.json` 中的布尔标志决定包含或排除各段。`INPUTS` 块中的 `inputs = [...]` 行根据启用的采集类型动态计算。模板中 `__ELK_FILE_PATH__` 占位符被替换为配置路径。

**注意**：ELK 日志不经过 Vector 的 ClickHouse sink，而是由独立的 Go processor 直接写入。因此当仅有 ELK 启用时，ClickHouse sink 不会出现在生成的配置中。

## ClickHouse 26.6 已知问题

1. **配置文件损坏**：预处理后的 `config.xml` 会出现大量重复注释和空字节。修复：`start.sh` 启动前自动用 `config_minimal.xml` 覆盖。该备份文件需包含 `<path>`、`<http_port>`、`<profiles>`、`<users>` 和 LMS_Logs 表所需的 `<storage_configuration>`（hot_warm_cold 策略）。

2. **聚合别名冲突**：`max(Timestamp) AS Timestamp` 与 `WHERE Timestamp` 冲突导致 `ILLEGAL_AGGREGATION`。修复：SQL 中别名改为 `TS`，Go 中重命名为 `Timestamp`。

3. **DELETE 异步**：`ALTER TABLE ... DELETE` 立即返回但可能需数秒生效。用 `OPTIMIZE TABLE ... FINAL` 强制完成。

## 采集器页面

- **采集源开关**：点击 Linux系统日志 / 网络设备日志 / ELK本地日志文件 标签可直接启用/停用对应采集源，触发 Vector 配置重新生成和重启
- **采集器启停**：启用/停用按钮控制整个采集器的开关
- 切换采集源时显示加载缓冲页面，操作完成后自动刷新

## Kafka topic

仅保留一个 topic `lms_elk_logs`（6 分区，zstd 压缩，24h 保留），数据流：Vector(file) → Kafka → Processor(Go) → ClickHouse。旧架构的 `lms_elk_logs_clean` 已废弃删除。

## 前端特性

- **日历选择器**：`app.js` 中自定义原生 JS 日历（initDatePickers、showDP、renderCalendar），零依赖
- **筛选器**：级别、主机、来源下拉框选择后自动触发查询，无需手动点击查询按钮
- **页码跳转**：分页栏末尾有 `跳至 [__] 页` 输入框
- **日期查询**：选择日期后自动查询

## 重要约束

- **无构建步骤** — HTML/CSS/JS 直接使用，Go 需 `go build` 编译
- **无测试框架** — 项目中不存在任何测试
- **git 仓库** — https://github.com/yxp008/LMS
- **日志级别**存储为字符串，保留各日志源原始等级值，不做统一映射
- **SQL 注入风险** — API 通过字符串拼接构建 SQL。**不要**在新端点中引入类似模式
- **SMTP 凭据**以明文存储
- **相对路径** — 基于 `PROJECT_ROOT` 自动计算，支持跨机器移植
- **无认证** — 所有 API 端点对外开放
- **浏览器缓存** — 前端变更需 `Ctrl+F5` 强制刷新
