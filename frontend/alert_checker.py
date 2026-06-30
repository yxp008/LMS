#!/usr/bin/env python3
"""
LMS 告警检查器 - 定期检查告警规则并发送通知
"""

import json
import smtplib
import ssl
import time
import urllib.request
import urllib.parse
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from datetime import datetime
from pathlib import Path

CLICKHOUSE_URL = "http://localhost:8123"
DATABASE = "LMS"

# SMTP 配置（需要根据实际邮箱服务商修改）
SMTP_CONFIG = {
    "host": "smtp.qq.com",
    "port": 465,
    "use_ssl": True,
    "username": "",  # 发件人邮箱
    "password": "",  # SMTP授权码（非登录密码）
    "sender": "",    # 发件人邮箱
}

SMTP_CONFIG_FILE = Path(__file__).parent / "smtp_config.json"

# 存储上次告警时间，避免重复发送
last_alert_times = {}


def load_smtp_config():
    """从文件加载SMTP配置"""
    global SMTP_CONFIG
    if SMTP_CONFIG_FILE.exists():
        try:
            with open(SMTP_CONFIG_FILE, "r") as f:
                cfg = json.load(f)
                SMTP_CONFIG["host"] = cfg.get("host", "")
                SMTP_CONFIG["port"] = cfg.get("port", 465)
                SMTP_CONFIG["sender"] = cfg.get("sender", "")
                SMTP_CONFIG["username"] = cfg.get("sender", "")
                SMTP_CONFIG["password"] = cfg.get("password", "")
                SMTP_CONFIG["use_ssl"] = cfg.get("use_ssl", True)
                print(f"[ALERTER] 已加载SMTP配置: {cfg.get('host')}:{cfg.get('port')}")
        except Exception as e:
            print(f"[ALERTER] 加载SMTP配置失败: {e}")


def ch_query(sql):
    """执行ClickHouse查询"""
    data = sql.strip().encode("utf-8")
    req = urllib.request.Request(
        f"{CLICKHOUSE_URL}/?default_format=JSONEachRow",
        data=data, method="POST",
    )
    req.add_header("Content-Type", "text/plain")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            body = resp.read().decode("utf-8")
            if not body.strip():
                return []
            # 检查是否有错误
            if body.strip().startswith("Code:") or "DB::Exception" in body:
                print(f"[ALERTER] ClickHouse错误: {body.strip()[:200]}")
                return []
            return [json.loads(line) for line in body.strip().split("\n") if line.strip()]
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8", errors="replace")
        print(f"[ALERTER] ClickHouse HTTP错误 {e.code}: {error_body[:200]}")
        return []
    except Exception as e:
        print(f"[ALERTER] ClickHouse query error: {e}")
        return []


def parse_interval(interval_str):
    """解析间隔字符串（如 5m, 1h, 30s）为秒数"""
    if not interval_str:
        return 300
    unit = interval_str[-1]
    try:
        value = int(interval_str[:-1])
    except ValueError:
        return 300

    if unit == 's':
        return value
    elif unit == 'm':
        return value * 60
    elif unit == 'h':
        return value * 3600
    elif unit == 'd':
        return value * 86400
    return value


def send_email(to_addr, subject, body):
    """发送邮件通知"""
    if not SMTP_CONFIG["username"] or not SMTP_CONFIG["password"]:
        print(f"[ALERTER] SMTP未配置，邮件内容如下:")
        print(f"  收件人: {to_addr}")
        print(f"  主题: {subject}")
        print(f"  内容: {body[:200]}...")
        return False

    try:
        msg = MIMEMultipart("alternative")
        msg["Subject"] = subject
        msg["From"] = SMTP_CONFIG["sender"]
        msg["To"] = to_addr

        html_body = f"""
        <html>
        <body style="font-family: Arial, sans-serif; padding: 20px;">
            <h2 style="color: #e74c3c;">LMS 告警通知</h2>
            <div style="background: #f8f9fa; padding: 15px; border-radius: 8px; margin: 10px 0;">
                <pre style="white-space: pre-wrap; font-size: 14px;">{body}</pre>
            </div>
            <p style="color: #888; font-size: 12px;">此邮件由 LMS 日志管理系统自动发送</p>
        </body>
        </html>
        """

        msg.attach(MIMEText(body, "plain", "utf-8"))
        msg.attach(MIMEText(html_body, "html", "utf-8"))

        if SMTP_CONFIG["use_ssl"]:
            context = ssl.create_default_context()
            with smtplib.SMTP_SSL(SMTP_CONFIG["host"], SMTP_CONFIG["port"], context=context) as server:
                server.login(SMTP_CONFIG["username"], SMTP_CONFIG["password"])
                server.sendmail(SMTP_CONFIG["sender"], [to_addr], msg.as_string())
        else:
            with smtplib.SMTP(SMTP_CONFIG["host"], SMTP_CONFIG["port"]) as server:
                server.starttls()
                server.login(SMTP_CONFIG["username"], SMTP_CONFIG["password"])
                server.sendmail(SMTP_CONFIG["sender"], [to_addr], msg.as_string())

        print(f"[ALERTER] 邮件已发送至 {to_addr}: {subject}")
        return True
    except Exception as e:
        print(f"[ALERTER] 邮件发送失败: {e}")
        return False


def check_alert_rules():
    """检查所有启用的告警规则"""
    global last_alert_times

    rules = ch_query(f"SELECT * FROM {DATABASE}.LMS_AlertRules WHERE Status = '1'")

    for rule in rules:
        rule_id = rule.get("AlertRule_ID", "")
        name = rule.get("Name", "")
        alert_sql = rule.get("Alert_Sql", "")
        interval = rule.get("Interval", "5m")
        channel = rule.get("Channel", "1")
        address = rule.get("Address", "")
        level = rule.get("Level", "3")

        if not alert_sql or not address:
            continue

        # 检查是否到了发送间隔
        interval_sec = parse_interval(interval)
        now = time.time()
        last_time = last_alert_times.get(rule_id, 0)

        if now - last_time < interval_sec:
            continue

        # 执行告警SQL
        results = ch_query(alert_sql)

        if results:
            first_val = results[0]
            count = 0
            for v in first_val.values():
                try:
                    count = int(v)
                    break
                except (ValueError, TypeError):
                    continue

            if count > 0:
                # 触发告警
                level_map = {"1": "严重", "2": "高", "3": "中", "4": "低"}
                alert_msg = (
                    f"[LMS 告警] {name}\n"
                    f"告警等级: {level_map.get(level, level)}\n"
                    f"告警规则: {rule_id}\n"
                    f"查询结果: {count} 条记录匹配\n"
                    f"告警SQL: {alert_sql}\n"
                    f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}"
                )

                print(f"[ALERTER] 触发告警: {name} (匹配 {count} 条)")

                # 记录告警触发历史到数据库
                import time as _time
                trigger_id = f"T{int(_time.time()*1000)}"
                channel_map = {"1": "邮件", "2": "短信", "3": "Webhook"}
                safe_name = name.replace("'", "\\'")
                safe_address = address.replace("'", "\\\\'")
                safe_msg = f"告警触发: {safe_name} 匹配{count}条记录".replace("'", "\\'")
                insert_sql = f"""INSERT INTO {DATABASE}.LMS_AlertTriggers (Trigger_ID, AlertRule_ID, Rule_Name, Trigger_Time, Match_Count, Channel, Address, Message) VALUES ('{trigger_id}', '{rule_id}', '{safe_name}', now(), {count}, '{channel}', '{safe_address}', '{safe_msg}')"""
                print(f"[ALERTER] INSERT trigger: rule_id={rule_id} channel={channel_map.get(channel, channel)} count={count}")
                ch_query(insert_sql)

                if channel == "1":
                    send_email(address, f"[LMS告警] {name} - {level_map.get(level, level)}", alert_msg)
                elif channel == "3":
                    # Webhook (简单POST)
                    try:
                        webhook_data = json.dumps({"alert": name, "count": count, "level": level_map.get(level, level), "rule_id": rule_id}).encode()
                        req = urllib.request.Request(address, data=webhook_data, method="POST")
                        req.add_header("Content-Type", "application/json")
                        urllib.request.urlopen(req, timeout=10)
                        print(f"[ALERTER] Webhook 已发送至 {address}")
                    except Exception as e:
                        print(f"[ALERTER] Webhook 发送失败: {e}")

                last_alert_times[rule_id] = now
            else:
                print(f"[ALERTER] 规则 {name} 未触发（匹配 0 条）")
        else:
            print(f"[ALERTER] 规则 {name} 查询出错或返回空")


def main():
    print("=" * 50)
    print("  LMS 告警检查器已启动")
    print(f"  检查间隔: 10秒")
    load_smtp_config()
    print("=" * 50)

    while True:
        try:
            check_alert_rules()
        except Exception as e:
            print(f"[ALERTER] 检查出错: {e}")
        time.sleep(5)


if __name__ == "__main__":
    main()
