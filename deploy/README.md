# V4.1 分离部署指南

## 电脑 A — 客户端（采集器）

**准备：**
1. 复制项目到客户端
2. 安装 Vector：`curl https://sh.vector.dev | bash`
3. ELK 日志文件（.ndjson）放入 `collector/elk_logs/incoming/`

**启动：**
```bash
# 可选：指向服务端用于注册
export LMS_SERVER_URL=http://服务端IP:8080

bash deploy/client/start.sh
```

**配置采集器：**
1. 浏览器打开 `http://localhost:8081`（或修改 `config.env` 中 `COLLECTOR_PORT`）
2. 点击「注册采集器」→ 传输地址填 `服务端IP:9092` → 勾选采集源 → 保存
3. 保存后自动向服务端注册

---

## 电脑 B — 服务端（日志管理）

**准备：**
1. 复制项目到服务端
2. 安装 Kafka（解压到 `~/kafka/`，或修改 `config.env` 中 `KAFKA_HOME`）
3. ClickHouse 二进制放入 `data/clickhouse_data/`
4. 执行 `bash install.sh` 编译 Go 程序
5. 建表：`clickhouse-client < database_design/sql/LMS_Logs.sql`（依次执行所有 SQL）

**启动：**
```bash
bash deploy/server/start.sh
```

**访问：** `http://服务端IP:8080`（或修改 `config.env` 中 `SERVER_PORT`）

---

## 自定义端口

修改项目根目录的 `config.env`：

```bash
SERVER_PORT=8080
COLLECTOR_PORT=8081
CLICKHOUSE_PORT=8123
KAFKA_PORT=9092
```

---

## 网络要求

| 方向 | 端口 | 用途 |
|---|---|---|
| 客户端 → 服务端 | 9092 | 日志数据 (Kafka) |
| 客户端 → 服务端 | 8080 | 采集器注册 |
| 客户端本地 | 8081 | 采集器管理页面 |
| 服务端本地 | 8080 | 日志管理页面 |
