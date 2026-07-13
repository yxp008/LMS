#!/bin/bash
# LMS 本地全栈启动（服务端 + 客户端）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export LMS_PROJECT_ROOT="$SCRIPT_DIR"

bash "$SCRIPT_DIR/server/start_server.sh"
bash "$SCRIPT_DIR/client/start_client.sh"
