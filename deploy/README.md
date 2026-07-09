# LMS 部署指南

## 电脑 A — 客户端（采集器）

**需要的文件：**
```
deploy/client/
├── server              # Go 二进制（复制自 frontend/server，编译时指 -collector）
├── collector/          # 配置模板 + 运行时生成
│   └── vector_wsl.toml.template
└── start.sh
```

**还需要：**
- Vector: `~/.vector/bin/vector`（`curl https://sh.vector.dev | bash`）
- ELK 日志文件放入 `collector/elk_logs/incoming/`

**启动：**
```bash
export LMS_SERVER_URL=http://服务端IP:8080
bash start.sh
```

**配置采集器：**
1. 浏览器打开 `http://localhost:8081`
2. 点击「注册采集器」→ 传输地址填 `服务端IP:9092` → 勾选采集源 → 保存
3. 放入 .ndjson 文件到 `collector/elk_logs/incoming/` 开始采集

---

## 电脑 B — 服务端（日志管理）

**需要的文件：**
```
deploy/server/
├── server              # Go 二进制（复制自 frontend/server）
├── processor/processor # Go 二进制
├── data/clickhouse_data/  # ClickHouse + 配置
├── frontend/           # 前端静态文件 (index.html/app.js/style.css)
├── database_design/sql/ # 建表 SQL
└── start.sh
```

**还需要：**
- Kafka: `~/kafka/`
- ClickHouse 二进制在 `data/clickhouse_data/`

**首次启动：**
```bash
# 1. 建表
clickhouse-client --database LMS < database_design/sql/LMS_Logs.sql
# ... (依次执行所有 SQL)

# 2. 启动
bash start.sh
```

**访问：** `http://服务端IP:8080`

---

## 网络要求

| 方向 | 端口 | 说明 |
|---|---|---|
| 客户端 → 服务端 Kafka | 9092 | 日志数据传输 |
| 客户端 → 服务端 API | 8080 | 采集器注册 |
| 客户端本地 | 8081 | 采集器管理页面 |
| 服务端本地 | 8080 | 日志管理页面 |
