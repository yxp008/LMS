#!/usr/bin/env python3
"""
LMS 日志管理系统 - 后端API服务器
提供前端静态文件服务和ClickHouse查询API
"""

import json
import os
import re
import smtplib
import socket
import ssl
import subprocess
import time as _time
import urllib.request
import urllib.parse
from datetime import datetime
from http.server import HTTPServer, SimpleHTTPRequestHandler
from pathlib import Path

CLICKHOUSE_URL = "http://localhost:8123"
DATABASE = "LMS"
TABLE = "LMS_Logs"
SMTP_CONFIG_FILE = Path(__file__).parent / "smtp_config.json"

FRONTEND_DIR = Path(__file__).parent
PROJECT_DIR = Path(__file__).parent.parent
COLLECTION_PREFS_FILE = PROJECT_DIR / "collector" / "collection_prefs.json"
VECTOR_TEMPLATE = PROJECT_DIR / "collector" / "vector_wsl.toml.template"
VECTOR_CONFIG = PROJECT_DIR / "collector" / "vector_wsl.toml"
VECTOR_BIN = "/home/yxp/.vector/bin/vector"
VECTOR_PID_FILE = Path("/tmp/vector.pid")
VECTOR_LOG = "/tmp/vector.log"


def ch_query(sql):
    """执行ClickHouse查询并返回结果"""
    data = sql.strip().encode("utf-8")
    req = urllib.request.Request(
        f"{CLICKHOUSE_URL}/?default_format=JSONEachRow",
        data=data,
        method="POST",
    )
    req.add_header("Content-Type", "text/plain")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            body = resp.read().decode("utf-8")
            if not body.strip():
                return []
            results = []
            for line in body.strip().split("\n"):
                if line.strip():
                    results.append(json.loads(line))
            return results
    except Exception as e:
        print(f"ClickHouse query error: {e}")
        return []


def ch_query_raw(sql):
    """执行ClickHouse查询并返回原始文本"""
    data = sql.strip().encode("utf-8")
    req = urllib.request.Request(
        f"{CLICKHOUSE_URL}/?default_format=JSONEachRow",
        data=data,
        method="POST",
    )
    req.add_header("Content-Type", "text/plain")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.read().decode("utf-8")
    except Exception as e:
        print(f"ClickHouse query error: {e}")
        return ""


def load_collection_prefs():
    """加载采集偏好配置"""
    defaults = {"linux_system_logs": True, "network_device_logs": True, "elk_file_logs": False, "elk_file_path": "/home/yxp/LMS_mimo/collector/elk_logs/incoming/"}
    if COLLECTION_PREFS_FILE.exists():
        try:
            with open(COLLECTION_PREFS_FILE) as f:
                prefs = json.load(f)
                defaults.update(prefs)
        except Exception:
            pass
    return defaults


def save_collection_prefs(prefs):
    """保存采集偏好配置"""
    COLLECTION_PREFS_FILE.parent.mkdir(parents=True, exist_ok=True)
    tmp = COLLECTION_PREFS_FILE.with_suffix(".json.tmp")
    with open(tmp, "w") as f:
        json.dump(prefs, f, indent=2)
    tmp.replace(COLLECTION_PREFS_FILE)


def generate_vector_config(prefs):
    """根据采集偏好从模板生成 vector_wsl.toml"""
    template_text = VECTOR_TEMPLATE.read_text()
    output_lines = []
    lines = template_text.split("\n")
    i = 0

    while i < len(lines):
        line = lines[i]

        coll_match = re.match(r'^# ===== COLLECTION:\s*(\w+)\s*=====$', line)
        if coll_match:
            coll_type = coll_match.group(1)
            i += 1
            block_lines = []
            while i < len(lines) and not re.match(r'^# ===== END COLLECTION\s*=====$', lines[i]):
                block_lines.append(lines[i])
                i += 1
            i += 1
            if prefs.get(coll_type, True):
                output_lines.extend(block_lines)
            continue

        inputs_match = re.match(r'^# ===== INPUTS\s*=====$', line)
        if inputs_match:
            i += 1
            # 先计算需要的 inputs
            inputs_list = []
            if prefs.get("linux_system_logs", True):
                inputs_list.append('"cleanup_journald"')
            if prefs.get("network_device_logs", True):
                inputs_list.append('"cleanup_syslog"')
            # elk 走 processor 直入库，不经过 Vector ClickHouse sink

            block_lines = []
            while i < len(lines) and not re.match(r'^# ===== END INPUTS\s*=====$', lines[i]):
                if re.match(r'^\s*inputs\s*=', lines[i]):
                    if inputs_list:
                        block_lines.append(f"inputs = [{', '.join(inputs_list)}]")
                    # inputs 为空时跳过这一行
                else:
                    if inputs_list:
                        block_lines.append(lines[i])
                i += 1
            i += 1
            output_lines.extend(block_lines)
            continue

        output_lines.append(line)
        i += 1

    VECTOR_CONFIG.parent.mkdir(parents=True, exist_ok=True)
    cfg_text = "\n".join(output_lines) + "\n"
    # 替换 ELK 文件路径占位符
    elk_path = prefs.get("elk_file_path", "/home/yxp/LMS_mimo/collector/elk_logs/")
    cfg_text = cfg_text.replace("__ELK_FILE_PATH__", elk_path)
    VECTOR_CONFIG.write_text(cfg_text)


def vector_is_running():
    """检查 Vector 进程是否在运行"""
    if VECTOR_PID_FILE.exists():
        try:
            pid = int(VECTOR_PID_FILE.read_text().strip())
            with open(f"/proc/{pid}/status") as sf:
                for line in sf:
                    if line.startswith("State:") and "Z" not in line:
                        return True
            return False
        except Exception:
            return False
    return False


def kill_all_vector():
    """强制终止所有 Vector 进程"""
    r = subprocess.run(["pgrep", "-x", "vector"], capture_output=True, text=True)
    pids = sorted([int(p.strip()) for p in r.stdout.strip().split("\n") if p.strip()])
    for pid in pids:
        try:
            os.kill(pid, 9)
        except Exception:
            pass
    if pids:
        subprocess.run(["sleep", "2"], capture_output=True)
    r2 = subprocess.run(["pgrep", "-x", "vector"], capture_output=True, text=True)
    for pid_str in r2.stdout.strip().split("\n"):
        pid_str = pid_str.strip()
        if pid_str and int(pid_str) not in pids:
            try:
                os.kill(int(pid_str), 9)
            except Exception:
                pass


def stop_vector():
    """停止 Vector"""
    kill_all_vector()
    try:
        VECTOR_PID_FILE.unlink()
    except Exception:
        pass
    ch_query(f"ALTER TABLE {DATABASE}.LMS_Collectors UPDATE Status = '0' WHERE 1=1")
    print("[COLLECTOR] Vector 已停止")


def start_vector():
    """启动 Vector"""
    proc = subprocess.Popen(
        [VECTOR_BIN, "--config", str(VECTOR_CONFIG)],
        stdout=open(VECTOR_LOG, "a"),
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    subprocess.run(["sleep", "3"], capture_output=True)
    r = subprocess.run(["pgrep", "-x", "vector"], capture_output=True, text=True)
    real_pid = r.stdout.strip().split("\n")[0] if r.stdout.strip() else str(proc.pid)
    VECTOR_PID_FILE.write_text(str(real_pid))
    ch_query(f"ALTER TABLE {DATABASE}.LMS_Collectors UPDATE Status = '1' WHERE 1=1")
    print(f"[COLLECTOR] Vector 已启用, PID={real_pid}")


def restart_vector(prefs):
    """根据偏好重启 Vector（生成配置→停→启）"""
    any_enabled = prefs.get("linux_system_logs", False) or prefs.get("network_device_logs", False) or prefs.get("elk_file_logs", False)
    if any_enabled:
        generate_vector_config(prefs)
        stop_vector()
        start_vector()
    else:
        stop_vector()
        ch_query(f"ALTER TABLE {DATABASE}.LMS_Collectors UPDATE Status = '0' WHERE 1=1")
        print("[COLLECTOR] 所有采集类型已禁用，Vector 不启动")


class LMSHandler(SimpleHTTPRequestHandler):
    """处理HTTP请求的Handler"""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=str(FRONTEND_DIR), **kwargs)

    def do_GET(self):
        if self.path.startswith("/api/"):
            self._handle_api()
        elif self.path == "/" or self.path == "":
            self.path = "/index.html"
            super().do_GET()
        else:
            super().do_GET()

    def do_POST(self):
        if self.path.startswith("/api/"):
            self._handle_api_post()
        else:
            self.send_error(404)

    def _handle_api(self):
        """处理GET API请求"""
        path = self.path.split("?")[0]

        if path == "/api/stats":
            self._api_stats()
        elif path == "/api/logs":
            self._api_logs()
        elif path == "/api/levels":
            self._api_levels()
        elif path == "/api/timeline":
            self._api_timeline()
        elif path == "/api/sources":
            self._api_sources()
        elif path == "/api/hosts":
            self._api_hosts()
        elif path == "/api/collectors":
            self._api_collectors()
        elif path == "/api/alert-rules":
            self._api_alert_rules()
        elif path == "/api/smtp-config":
            self._api_smtp_config_get()
        elif path == "/api/sql-validate":
            self._api_sql_validate_get()
        elif path == "/api/alert-triggers":
            self._api_alert_triggers()
        elif path == "/api/collection-prefs":
            self._api_collection_prefs_get()
        else:
            self.send_error(404)

    def _handle_api_post(self):
        """处理POST API请求"""
        path = self.path.split("?")[0]
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        try:
            req_data = json.loads(body)
        except Exception:
            self._json_response(400, {"error": "invalid json"})
            return

        if path == "/api/query":
            sql = req_data.get("sql", "")
            results = ch_query(sql)
            self._json_response(200, results)
        elif path == "/api/alert-rules":
            self._api_alert_rules_post(req_data)
        elif path == "/api/smtp-config":
            self._api_smtp_config_post(req_data)
        elif path == "/api/smtp-test":
            self._api_smtp_test(req_data)
        elif path == "/api/sql-validate":
            self._api_sql_validate(req_data)
        elif path == "/api/webhook":
            self._api_webhook(req_data)
        elif path == "/api/collectors":
            self._api_collectors_post(req_data)
        elif path == "/api/collection-prefs":
            self._api_collection_prefs_post(req_data)
        else:
            self.send_error(404)

    def _api_stats(self):
        """仪表盘统计概览"""
        results = ch_query(f"""
            SELECT
                count() as total_logs,
                countIf(Level = '1') as info_count,
                countIf(Level = '2') as warn_count,
                countIf(Level = '3') as error_count,
                countIf(Level = '4') as debug_count,
                countIf(Timestamp >= now() - INTERVAL 1 DAY) as last_24h,
                countIf(Timestamp >= now() - INTERVAL 1 HOUR) as last_1h,
                min(Timestamp) as earliest,
                max(Timestamp) as latest,
                uniq(Host) as host_count,
                uniq(Source_Type) as source_count
            FROM {DATABASE}.{TABLE}
        """)
        if results:
            self._json_response(200, results[0])
        else:
            self._json_response(200, {
                "total_logs": 0, "info_count": 0, "warn_count": 0,
                "error_count": 0, "debug_count": 0, "last_24h": 0,
                "last_1h": 0, "earliest": "", "latest": "",
                "host_count": 0, "source_count": 0
            })

    def _api_logs(self):
        """查询日志列表"""
        params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
        page = int(params.get("page", ["1"])[0])
        page_size = int(params.get("page_size", ["50"])[0])
        level = params.get("level", [""])[0]
        host = params.get("host", [""])[0]
        source_type = params.get("source_type", [""])[0]
        search = params.get("search", [""])[0]
        start_time = params.get("start_time", [""])[0]
        end_time = params.get("end_time", [""])[0]

        offset = (page - 1) * page_size
        conditions = []
        if level:
            conditions.append(f"Level = '{level}'")
        if host:
            conditions.append(f"Host = '{host}'")
        if source_type:
            conditions.append(f"Source_Type = '{source_type}'")
        if search:
            conditions.append(f"Message LIKE '%{search}%'")
        if start_time:
            conditions.append(f"Timestamp >= '{start_time}'")
        if end_time:
            conditions.append(f"Timestamp <= '{end_time}'")

        where = "WHERE " + " AND ".join(conditions) if conditions else ""

        total_results = ch_query(f"SELECT count() as total FROM {DATABASE}.{TABLE} {where}")
        total = total_results[0]["total"] if total_results else 0

        logs = ch_query(f"""
            SELECT Log_ID, Timestamp, Level, Host, Source_Type, Message, Tags, Collector_ID
            FROM {DATABASE}.{TABLE}
            {where}
            ORDER BY Timestamp DESC
            LIMIT {page_size} OFFSET {offset}
        """)

        self._json_response(200, {
            "total": total,
            "page": page,
            "page_size": page_size,
            "total_pages": (total + page_size - 1) // page_size if total > 0 else 0,
            "data": logs
        })

    def _api_levels(self):
        """按日志级别统计"""
        results = ch_query(f"""
            SELECT Level, count() as count
            FROM {DATABASE}.{TABLE}
            GROUP BY Level
            ORDER BY Level
        """)
        self._json_response(200, results)

    def _api_timeline(self):
        """按时间线统计日志数量"""
        params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
        hours = int(params.get("hours", ["24"])[0])

        results = ch_query(f"""
            SELECT
                toStartOfInterval(Timestamp, INTERVAL {max(1, hours // 24)} HOUR) as time_point,
                Level,
                count() as count
            FROM {DATABASE}.{TABLE}
            WHERE Timestamp >= now() - INTERVAL {hours} HOUR
            GROUP BY time_point, Level
            ORDER BY time_point
        """)
        self._json_response(200, results)

    def _api_sources(self):
        """按来源类型统计（含已启用但无数据的来源类型）"""
        results = ch_query(f"""
            SELECT Source_Type, count() as count
            FROM {DATABASE}.{TABLE}
            GROUP BY Source_Type
            ORDER BY count DESC
        """)
        # 补全所有已知来源类型（含暂无数据的）
        existing = {r["Source_Type"] for r in results}
        for t in ["Linux系统日志", "网络设备日志", "ELK本地日志文件"]:
            if t not in existing:
                results.append({"Source_Type": t, "count": 0})
        self._json_response(200, results)

    def _api_hosts(self):
        """按主机统计"""
        results = ch_query(f"""
            SELECT Host, count() as count
            FROM {DATABASE}.{TABLE}
            GROUP BY Host
            ORDER BY count DESC
            LIMIT 20
        """)
        self._json_response(200, results)

    def _api_collectors(self):
        """查询采集器列表"""
        results = ch_query(f"SELECT * FROM {DATABASE}.LMS_Collectors ORDER BY Collector_ID")
        prefs = load_collection_prefs()
        any_enabled = prefs.get("linux_system_logs", False) or prefs.get("network_device_logs", False) or prefs.get("elk_file_logs", False)
        actual_status = "1" if (any_enabled and vector_is_running()) else "0"
        for r in results:
            r["Status"] = actual_status
            r["Source_Types"] = [
                {"name": "Linux系统日志", "enabled": prefs.get("linux_system_logs", True)},
                {"name": "网络设备日志", "enabled": prefs.get("network_device_logs", True)},
                {"name": "ELK本地日志文件", "enabled": prefs.get("elk_file_logs", False)},
            ]
        self._json_response(200, results)

    def _api_collectors_post(self, data):
        """更新采集器状态并控制Vector进程（兼容旧接口）"""
        action = data.get("action", "")
        if action == "update_status":
            collector_id = data.get("Collector_ID", "")
            status = data.get("Status", "1")
            if not collector_id:
                self._json_response(400, {"error": "missing Collector_ID"})
                return
            ch_query(f"ALTER TABLE {DATABASE}.LMS_Collectors UPDATE Status = '{status}' WHERE Collector_ID = '{collector_id}'")

            if status == "0":
                stop_vector()
                print(f"[COLLECTOR] 采集器 {collector_id} 已停用")
            elif status == "1":
                prefs = load_collection_prefs()
                generate_vector_config(prefs)
                start_vector()
                print(f"[COLLECTOR] 采集器 {collector_id} 已启用")

            self._json_response(200, {"ok": True})
        else:
            self._json_response(400, {"error": "unknown action"})

    def _api_alert_rules(self):
        """查询告警规则列表"""
        results = ch_query(f"SELECT * FROM {DATABASE}.LMS_AlertRules ORDER BY AlertRule_ID")
        self._json_response(200, results)

    def _api_alert_rules_post(self, data):
        """创建/更新/删除告警规则"""
        action = data.get("action", "")

        if action == "create":
            name = data.get("Name", "").replace("'", "\\'")
            desc = data.get("Desc", "").replace("'", "\\'")
            alert_sql = data.get("Alert_Sql", "").replace("'", "\\'")
            interval = data.get("Interval", "").replace("'", "\\'")
            channel = data.get("Channel", "1")
            address = data.get("Address", "").replace("'", "\\'")
            level = data.get("Level", "3")
            status = data.get("Status", "1")

            existing = ch_query(f"SELECT max(AlertRule_ID) as max_id FROM {DATABASE}.LMS_AlertRules")
            max_num = 1
            if existing and existing[0].get("max_id"):
                max_id = existing[0]["max_id"]
                max_num = int(max_id.replace("AR", "")) + 1
            rule_id = f"AR{max_num:03d}"

            ch_query(f"""
                INSERT INTO {DATABASE}.LMS_AlertRules
                (AlertRule_ID, Name, Desc, Alert_Sql, Interval, Channel, Address, Created_Time, Updated_Time, Level, Status)
                VALUES ('{rule_id}', '{name}', '{desc}', '{alert_sql}', '{interval}', '{channel}', '{address}', now(), now(), '{level}', '{status}')
            """)
            self._json_response(200, {"ok": True, "AlertRule_ID": rule_id})

        elif action == "update":
            rule_id = data.get("AlertRule_ID", "")
            if not rule_id:
                self._json_response(400, {"error": "missing AlertRule_ID"})
                return

            fields = []
            for key in ["Name", "Desc", "Alert_Sql", "Interval", "Channel", "Address", "Level", "Status"]:
                if key in data:
                    val = str(data[key]).replace("'", "\\'")
                    fields.append(f"{key} = '{val}'")
            fields.append("Updated_Time = now()")

            ch_query(f"""
                ALTER TABLE {DATABASE}.LMS_AlertRules
                UPDATE {', '.join(fields)}
                WHERE AlertRule_ID = '{rule_id}'
            """)
            self._json_response(200, {"ok": True})

        elif action == "delete":
            rule_id = data.get("AlertRule_ID", "")
            if not rule_id:
                self._json_response(400, {"error": "missing AlertRule_ID"})
                return
            ch_query(f"DELETE FROM {DATABASE}.LMS_AlertRules WHERE AlertRule_ID = '{rule_id}'")
            self._json_response(200, {"ok": True})

        else:
            self._json_response(400, {"error": "unknown action"})

    def _api_smtp_config_get(self):
        """获取SMTP配置"""
        if SMTP_CONFIG_FILE.exists():
            try:
                with open(SMTP_CONFIG_FILE, "r") as f:
                    cfg = json.load(f)
                    cfg["password"] = ""
                    self._json_response(200, cfg)
                    return
            except Exception:
                pass
        self._json_response(200, {"host": "", "port": 465, "sender": "", "password": ""})

    def _api_smtp_config_post(self, data):
        """保存SMTP配置"""
        try:
            cfg = {
                "host": data.get("host", ""),
                "port": int(data.get("port", 465)),
                "sender": data.get("sender", ""),
                "password": data.get("password", ""),
                "use_ssl": True,
            }
            with open(SMTP_CONFIG_FILE, "w") as f:
                json.dump(cfg, f, indent=2)
            self._json_response(200, {"ok": True})
        except Exception as e:
            self._json_response(500, {"error": str(e)})

    def _api_smtp_test(self, data):
        """测试SMTP连接"""
        host = data.get("host", "")
        port = int(data.get("port", 465))
        sender = data.get("sender", "")
        password = data.get("password", "")

        if not host or not sender or not password:
            self._json_response(400, {"error": "请填写所有必填字段"})
            return

        try:
            context = ssl.create_default_context()
            with smtplib.SMTP_SSL(host, port, timeout=10, context=context) as server:
                server.login(sender, password)
            self._json_response(200, {"ok": True})
        except smtplib.SMTPAuthenticationError as e:
            self._json_response(200, {"ok": False, "error": f"认证失败: 请检查邮箱和授权码是否正确"})
        except smtplib.SMTPConnectError as e:
            self._json_response(200, {"ok": False, "error": f"连接失败: 无法连接到 {host}:{port}"})
        except socket.timeout:
            self._json_response(200, {"ok": False, "error": f"连接超时: {host}:{port} 无响应"})
        except Exception as e:
            self._json_response(200, {"ok": False, "error": str(e)})

    def _api_sql_validate(self, data):
        sql = data.get("sql", "").strip()
        if not sql:
            self._json_response(200, {"ok": False, "error": "SQL empty"})
            return
        test_sql = f"SELECT * FROM ({sql}) LIMIT 1 FORMAT JSONEachRow"
        req = urllib.request.Request(
            f"{CLICKHOUSE_URL}/?default_format=JSONEachRow",
            data=test_sql.encode("utf-8"), method="POST",
        )
        req.add_header("Content-Type", "text/plain")
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                resp.read()
                self._json_response(200, {"ok": True, "message": "SQL OK"})
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            msg = body.split("DB::Exception:")[1].split("(version")[0].strip() if "DB::Exception:" in body else str(e)
            self._json_response(200, {"ok": False, "error": msg})
        except Exception as e:
            self._json_response(200, {"ok": False, "error": str(e)})

    def _api_sql_validate_get(self):
        self._json_response(200, {"message": "请使用POST方法"})

    def _api_webhook(self, data):
        """Webhook接收端点"""
        import time as _time
        trigger_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        alert_name = data.get("alert", "未知告警").replace("'", "\\'")
        count = data.get("count", 0)
        level = data.get("level", "")
        rule_id = data.get("rule_id", "")

        print(f"[WEBHOOK] 收到告警: {alert_name}, 匹配 {count} 条, 等级 {level}, 规则 {rule_id}")

        safe_msg = f"Webhook告警触发: {alert_name} 匹配{count}条".replace("'", "\\'")
        ch_query(f"""INSERT INTO {DATABASE}.LMS_AlertTriggers
            (Trigger_ID, AlertRule_ID, Rule_Name, Trigger_Time, Match_Count, Channel, Address, Message)
            VALUES ('T{int(_time.time()*1000)}', '{rule_id}', '{alert_name}', now(), {count}, '3', 'webhook', '{safe_msg}')
        """)

        self._json_response(200, {"ok": True, "message": "Webhook received"})

    def _api_alert_triggers(self):
        """查询告警规则及触发次数（以规则为主体）"""
        results = ch_query(f"""
            SELECT
                r.AlertRule_ID,
                r.Name as Rule_Name,
                r.Desc,
                r.Alert_Sql,
                r.Interval,
                r.Channel,
                r.Address,
                r.Level,
                r.Status,
                count(t.Trigger_ID) as Trigger_Count,
                max(t.Trigger_Time) as Latest_Time,
                max(t.Match_Count) as Last_Match_Count
            FROM {DATABASE}.LMS_AlertRules r
            LEFT JOIN {DATABASE}.LMS_AlertTriggers t ON r.AlertRule_ID = t.AlertRule_ID
            GROUP BY r.AlertRule_ID, r.Name, r.Desc, r.Alert_Sql, r.Interval, r.Channel, r.Address, r.Level, r.Status
            ORDER BY Trigger_Count DESC, r.AlertRule_ID
        """)
        self._json_response(200, results)

    def _api_collection_prefs_get(self):
        """获取采集偏好配置"""
        prefs = load_collection_prefs()
        self._json_response(200, prefs)

    def _api_collection_prefs_post(self, data):
        """更新采集偏好配置并重启Vector"""
        prefs = load_collection_prefs()
        for key in ("linux_system_logs", "network_device_logs", "elk_file_logs"):
            if key in data:
                prefs[key] = bool(data[key])
        if "elk_file_path" in data and data["elk_file_path"]:
            prefs["elk_file_path"] = data["elk_file_path"]
        save_collection_prefs(prefs)
        restart_vector(prefs)
        any_enabled = prefs.get("linux_system_logs", False) or prefs.get("network_device_logs", False) or prefs.get("elk_file_logs", False)
        self._json_response(200, {"ok": True, "vector_running": any_enabled})

    def _json_response(self, code, data):
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(json.dumps(data, ensure_ascii=False, default=str).encode("utf-8"))

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.end_headers()

    def log_message(self, format, *args):
        print(f"[LMS] {args[0]}")


if __name__ == "__main__":
    PORT = 8080

    # 启动时根据采集偏好启动 Vector
    try:
        prefs = load_collection_prefs()
        any_enabled = prefs.get("linux_system_logs", False) or prefs.get("network_device_logs", False) or prefs.get("elk_file_logs", False)
        if any_enabled:
            generate_vector_config(prefs)
            start_vector()
        else:
            print("[INIT] 所有采集类型已禁用，Vector 不启动")
    except Exception as e:
        print(f"[INIT] 启动 Vector 失败: {e}")

    server = HTTPServer(("0.0.0.0", PORT), LMSHandler)
    print(f"LMS 前端服务已启动: http://localhost:{PORT}")
    print(f"ClickHouse: {CLICKHOUSE_URL}")
    server.serve_forever()
