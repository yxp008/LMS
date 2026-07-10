#!/bin/bash
# LMS 客户端安装脚本 — 编译采集器组件
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "========================================"
echo "  LMS 客户端安装"
echo "========================================"

# 1. Go
echo ""
echo "[1/4] 检查 Go..."
if command -v go &> /dev/null; then
    echo "  -> Go $(go version | awk '{print $3}') 已安装"
else
    echo "  -> 未找到 Go，正在安装..."
    sudo apt-get update -qq && sudo apt-get install -y -qq golang-go
    echo "  -> Go 安装完成"
fi

# 2. 编译 server
echo ""
echo "[2/4] 编译 Server（Web 服务 + 采集器管理）..."
cd "$PROJECT_ROOT/frontend"
GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o server server.go collector_state.go 2>/dev/null || {
    GOPROXY=https://goproxy.cn,direct go build -o server server.go collector_state.go
}
echo "  -> server 编译成功"

# 3. 编译 reader
echo ""
echo "[3/4] 编译 Reader（JSON数组 → NDJSON）..."
cd "$PROJECT_ROOT/client/collector/elk_logs"
if [ -f reader.go ]; then
    GOPROXY=${GOPROXY:-https://goproxy.cn,direct} go build -o reader reader.go 2>/dev/null || {
        GOPROXY=https://goproxy.cn,direct go build -o reader reader.go
    }
    echo "  -> reader 编译成功"
else
    echo "  -> 跳过（无 reader.go）"
fi

# 4. 检查外部依赖
echo ""
echo "[4/4] 检查外部依赖..."
VECTOR_PATH="$HOME/.vector/bin/vector"
if [ -f "$VECTOR_PATH" ]; then
    echo "  -> Vector: $VECTOR_PATH ✓"
else
    echo "  -> Vector: 未找到（请执行 curl --proto '=https' -sSf https://sh.vector.dev | bash）"
fi

# 初始化配置
if [ ! -f "$PROJECT_ROOT/client/collector/collection_prefs.json" ]; then
    cat > "$PROJECT_ROOT/client/collector/collection_prefs.json" << 'EOF'
{
  "linux_system_logs": false,
  "network_device_logs": false,
  "elk_file_logs": true,
  "elk_file_path": "__ELK_FILE_PATH__"
}
EOF
    sed -i "s|__ELK_FILE_PATH__|$PROJECT_ROOT/client/collector/elk_logs/incoming/|g" "$PROJECT_ROOT/client/collector/collection_prefs.json"
    echo "  -> collection_prefs.json 已创建"
fi

mkdir -p "$PROJECT_ROOT/client/collector/vector_data" "$PROJECT_ROOT/logs"

echo ""
echo "========================================"
echo "  客户端安装完成"
echo ""
echo "  1. 编辑 client/config_client.env，设服务端 IP"
echo "  2. 启动: bash client/start_client.sh"
echo "========================================"
