# LMS Docker 部署

## 快速开始

```bash
# 全栈部署（服务端 + 客户端 + Kafka + ClickHouse）
cd docker
docker compose up -d

# 仅服务端
docker compose -f docker-compose.server.yml up -d

# 仅客户端（需指定服务端地址）
LMS_KAFKA_BROKER=服务端IP:9092 LMS_SERVER_URL=http://服务端IP:8080 \
  docker compose -f docker-compose.client.yml up -d
```

## 访问

| 服务 | 地址 |
|---|---|
| 服务端 Web | `http://localhost:8080` |
| 客户端 Web | `http://localhost:8081` |
| ClickHouse | `http://localhost:8123` |

## 服务说明

| 容器 | 作用 |
|---|---|
| `kafka` | 消息队列缓冲 |
| `clickhouse` | 列式数据库存储 |
| `server` | Go Web 服务 + 告警 |
| `processor` | Kafka → 脱敏 → ClickHouse |
| `collector` | 采集器管理界面 |
| `vector` | 日志采集引擎 |

## 自定义配置

通过环境变量覆盖默认值：

```bash
LMS_CLICKHOUSE_URL=http://your-clickhouse:8123
LMS_KAFKA_BROKER=your-kafka:9092
LMS_SERVER_URL=http://your-server:8080
```
