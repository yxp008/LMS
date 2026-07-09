#!/bin/bash
# LMS 客户端部署启动脚本
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
export LMS_PROJECT_ROOT="$PROJECT_ROOT"

echo "========================================"
echo "  LMS 采集器客户端"
echo "========================================"

# 启动采集器管理服务（含 Vector 自动管理）
if pgrep -f "server.*-collector" > /dev/null 2>&1; then
    echo "采集器服务已在运行"
else
    nohup $PROJECT_ROOT/frontend/server -collector > $PROJECT_ROOT/logs/collector.log 2>&1 &
    sleep 3
    echo "采集器服务已启动: http://localhost:8081"
fi

echo ""
echo "部署后请在浏览器打开 http://localhost:8081"
echo "  1. 点击「注册采集器」"
echo "  2. 传输地址填 服务端IP:9092"
echo "  3. 勾选采集源 → 保存"
echo ""
echo "环境变量（可选）:"
echo "  export LMS_SERVER_URL=http://服务端IP:8080  # 向服务端注册"
echo "========================================"
