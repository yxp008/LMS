#!/bin/bash
# ============================================
# LMS 日志管理系统 — 安装脚本
# ============================================
# 本脚本仅安装/编译项目自身组件，不修改系统配置。
#
# 卸载方法：
#   直接删除项目目录即可：rm -rf /安装路径/LMS_mimo
#   运行时数据（Kafka/ClickHouse）在项目子目录中一并删除。
#   若 Vector 安装在 ~/.vector/，需单独删除。
# ============================================

set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================"
echo "  LMS 安装脚本"
echo "========================================"

# ---------- 1. Go ----------
echo ""
echo "[1/7] 检查 Go..."
if command -v go &> /dev/null; then
    echo "  -> Go $(go version | awk '{print $3}') 已安装"
else
    echo "  -> 未找到 Go，正在安装..."
    sudo apt-get update -qq && sudo apt-get install -y -qq golang-go
    echo "  -> Go 安装完成"
fi

# ---------- 2. 编译 processor ----------
echo ""
echo "[2/7] 编译 Processor（Kafka → 脱敏 → ClickHouse）..."
cd "$SCRIPT_DIR/server/processor"
if command -v go &> /dev/null; then
    GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o processor . 2>/dev/null || {
        GOPROXY=https://goproxy.cn,direct go build -o processor .
    }
    echo "  -> processor 编译成功"
else
    echo "  -> 跳过（Go 不可用）"
fi

# ---------- 3. 编译 server ----------
echo ""
echo "[3/7] 编译 Server（Go Web 服务 + 告警）..."
cd "$SCRIPT_DIR/frontend"
if command -v go &> /dev/null; then
    GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o server server.go 2>/dev/null || {
        GOPROXY=https://goproxy.cn,direct go build -o server server.go
    }
    echo "  -> server 编译成功"
else
    echo "  -> 跳过（Go 不可用）"
fi

# ---------- 4. 编译 reader ----------
echo ""
echo "[4/7] 编译 Reader（JSON数组 → NDJSON 转换）..."
cd "$SCRIPT_DIR/client/collector/elk_logs"
if command -v go &> /dev/null && [ -f reader.go ]; then
    GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o reader reader.go 2>/dev/null || {
        GOPROXY=https://goproxy.cn,direct go build -o reader reader.go
    }
    echo "  -> reader 编译成功"
else
    echo "  -> 跳过"
fi

# ---------- 4. 检查外部依赖 ----------
echo ""
echo "[5/7] 检查外部依赖..."

# Vector
VECTOR_PATH="$HOME/.vector/bin/vector"
if [ -f "$VECTOR_PATH" ]; then
    echo "  -> Vector: $VECTOR_PATH ✓"
else
    echo "  -> Vector: 未找到（请执行 curl --proto '=https' -sSf https://sh.vector.dev | bash）"
fi

# ClickHouse
CH_PATH="$SCRIPT_DIR/data/clickhouse_data/clickhouse"
if [ -f "$CH_PATH" ]; then
    echo "  -> ClickHouse: $CH_PATH ✓"
else
    echo "  -> ClickHouse: 未找到（请将 clickhouse 二进制放入 server/data/clickhouse_data/）"
fi

# Kafka
KAFKA_PATH="$HOME/kafka/bin/kafka-server-start.sh"
if [ -f "$KAFKA_PATH" ]; then
    echo "  -> Kafka: $HOME/kafka/ ✓"
else
    echo "  -> Kafka: 未找到（请下载解压到 ~/kafka/）"
fi

# ---------- 5. 初始化配置 ----------
echo ""
echo "[6/7] 初始化配置..."

# 确保默认采集配置存在
if [ ! -f "$SCRIPT_DIR/client/collector/collection_prefs.json" ]; then
    cat > "$SCRIPT_DIR/client/collector/collection_prefs.json" << 'EOF'
{
  "linux_system_logs": false,
  "network_device_logs": false,
  "elk_file_logs": true,
  "elk_file_path": "__ELK_FILE_PATH__"
}
EOF
    # 替换为实际路径
    sed -i "s|__ELK_FILE_PATH__|$SCRIPT_DIR/client/collector/elk_logs/incoming/|g" "$SCRIPT_DIR/client/collector/collection_prefs.json"
    echo "  -> collection_prefs.json 已创建"
else
    echo "  -> collection_prefs.json 已存在，跳过"
fi

# 生成 Vector 配置
cd "$SCRIPT_DIR"
# Vector 配置由 Go server 启动时自动生成，无需额外步骤
echo "  -> Vector 配置将在服务启动时自动生成"

mkdir -p "$SCRIPT_DIR/client/collector/vector_data" "$SCRIPT_DIR/logs"
	echo "  -> 运行时目录已创建"

# ---------- 完成 ----------
echo ""
echo "========================================"
echo "  安装完成"
echo "========================================"
echo ""
echo "启动系统:   bash start.sh"
echo "停止系统:   bash stop.sh"
echo ""
echo "卸载方法:"
echo "  1. bash stop.sh"
echo "  2. rm -rf $SCRIPT_DIR"
echo "  3. rm -rf ~/.vector  (如果安装了 Vector)"
echo ""
echo "已安装的项目组件:"
echo "  - server/processor/processor  (Go 二进制)"
echo "  - client/collector/elk_logs/reader  (Go 二进制)"
echo "  - collection_prefs.json (配置文件)"
echo "  - vector_wsl.toml (Vector 配置)"
echo ""
echo "环境依赖（需单独安装，不在本脚本范围）:"
echo "  - Vector:     ~/.vector/bin/vector"
echo "  - ClickHouse: server/data/clickhouse_data/clickhouse"
echo "  - Kafka:      ~/kafka/"
echo "  - Go:         golang-go (已自动安装)"
echo "  - Python:     系统自带"
echo "========================================"
