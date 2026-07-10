# LMS 日志管理系统

> V4.7 — 五层架构，Go 全栈，Kafka 缓冲，三级存储，客户端/服务端分离，交互式AI对话

集日志采集、存储、查询、分析、可视化、告警于一体的集中式日志管理系统。

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
| Vector | `curl --proto '=https' --tlsv1.2 -sSf https://sh.vector.dev \| bash` |
| Kafka | 下载解压到 `~/kafka/`（[kafka.apache.org](https://kafka.apache.org/downloads)） |
| ClickHouse | 下载二进制放入 `server/data/clickhouse_data/`（[clickhouse.com](https://packages.clickhouse.com/tgz/stable/)） |

## 拉取项目

**全量克隆**（开发/单机测试）：

```bash
git clone <repo-url>
cd LMS
bash install.sh
bash start.sh
```

**按角色拉取**（分离部署）：

```bash
# 客户端（只拉 client/ + frontend/ + test/）
git clone --filter=blob:none --sparse <repo-url>
cd LMS
git sparse-checkout set client/ frontend/ test/ install.sh README.md

# 服务端（只拉 server/ + frontend/ + test/）
git clone --filter=blob:none --sparse <repo-url>
cd LMS
git sparse-checkout set server/ frontend/ test/ install.sh README.md
```

## 从零部署

### 单机（全部在本机）

```bash
# 1. 安装前置依赖
# 2. 克隆项目
git clone <repo-url> && cd LMS
# 3. 安装
bash install.sh
# 4. 启动
bash start.sh
# 5. 访问
#    服务端: http://localhost:8080
#    客户端: http://localhost:8081
```

### 双机分离

**服务端**（运行 Kafka + ClickHouse + Processor + Web）：

```bash
git clone --filter=blob:none --sparse <repo-url>
cd LMS
git sparse-checkout set server/ frontend/ test/ install.sh README.md
bash install.sh

# 编辑配置（默认 localhost 即可）
vim server/config_server.env
bash server/start_server.sh
```

**客户端**（运行 Vector + 采集器管理）：

```bash
git clone --filter=blob:none --sparse <repo-url>
cd LMS
git sparse-checkout set client/ frontend/ test/ install.sh README.md
bash install.sh

# 修改三个地址指向服务端 IP
vim client/config_client.env
#   LMS_CLICKHOUSE_URL=http://<服务端IP>:8123
#   LMS_KAFKA_BROKER=<服务端IP>:9092
#   LMS_SERVER_URL=http://<服务端IP>:8080

bash client/start_client.sh
```

## 目录结构

```
LMS/
├── client/                          # 客户端（采集器）
│   ├── collector/                   # Vector 配置 + ELK reader
│   ├── config_client.env            # 客户端配置
│   └── start_client.sh              # 客户端启动
├── server/                          # 服务端
│   ├── processor/                   # Kafka→ClickHouse 处理程序
│   ├── data/clickhouse_data/        # ClickHouse 二进制+数据
│   ├── kafka/                       # Kafka 数据
│   ├── database_design/sql/         # 建表 SQL
│   ├── config_server.env            # 服务端配置
│   └── start_server.sh              # 服务端启动
├── frontend/                        # 共用（Go Web 服务 + SPA）
├── test/                            # 测试脚本
├── install.sh / start.sh / stop.sh
├── README.md / CLAUDE.md
```

## API

全部端点位于 `http://localhost:8080`，详见 `test/Test.md`。

## 许可证

MIT
