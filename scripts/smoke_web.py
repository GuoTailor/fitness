#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""网页端（H5）链路冒烟：短信登录 → 首页 → 课包 → 下单 → mock 支付 → 订单状态过滤。

前置：fitness-server 已启动（wechat.mock/pay.mock/sms.mock 均开），:8080 可访问。
用法：python scripts/smoke_web.py [--base http://127.0.0.1:8080]
说明：登录限流 10 次/分/IP，多次运行间请间隔 60s。
"""
import argparse
import json
import sys
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080"


def req(method, path, data=None, token=None):
    r = urllib.request.Request(BASE + path, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    body = json.dumps(data).encode() if data is not None else None
    try:
        with urllib.request.urlopen(r, body, timeout=10) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", "replace")
        print(f"  !! HTTP {e.code} {path}: {body[:200]}")
        raise


def step(name, ok, detail=""):
    mark = "PASS" if ok else "FAIL"
    print(f"[{mark}] {name}" + (f" —— {detail}" if detail else ""))
    if not ok:
        sys.exit(1)


def main():
    global BASE
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default=BASE)
    args = ap.parse_args()
    BASE = args.base.rstrip("/")

    print(f"==> 网页端冒烟（{BASE}）")

    # 1. 健康检查
    r = req("GET", "/healthz")
    step("健康检查", r["code"] == 0)

    # 2. 发送短信验证码（mock 通道回显 devCode）
    phone = "13800001111"
    r = req("POST", "/api/v1/auth/sms/send", {"phone": phone})
    dev_code = (r.get("data") or {}).get("devCode")
    step("发送短信验证码（mock 回显）", r["code"] == 0 and dev_code, f"code={dev_code}")

    # 3. 验证码登录（首次自动注册）
    r = req("POST", "/api/v1/auth/login/phone", {"phone": phone, "smsCode": dev_code})
    token = (r.get("data") or {}).get("token")
    user = (r.get("data") or {}).get("user") or {}
    step("短信登录", bool(token), f"user={user.get('id')} {user.get('nickname')}")

    # 4. 验证码用后作废（重放必须被拒）
    r2 = req("POST", "/api/v1/auth/login/phone", {"phone": phone, "smsCode": dev_code})
    step("验证码防重放", r2["code"] != 0, f"code={r2['code']} msg={r2.get('msg')}")

    # 5. 浏览首页与课包
    r = req("GET", "/api/v1/home")
    step("首页聚合", r["code"] == 0)
    r = req("GET", "/api/v1/packages?page=1&page_size=5")
    pkgs = (r.get("data") or {}).get("list") or []
    step("课包列表", r["code"] == 0 and len(pkgs) > 0, f"{len(pkgs)} 个课包")
    if not pkgs:
        print("  !! 无课包数据，请先跑 scripts/smoke_p3.py 造数")
        return
    sku_hint = ""
    detail = req("GET", f"/api/v1/packages/{pkgs[0]['id']}", token=token)
    skus = (detail.get("data") or {}).get("skus") or []
    if skus:
        sku_hint = f"sku={skus[0]['id']} ¥{skus[0]['price']/100:g}"

    # 6. 下单 + mock 支付（H5 native 通道）
    sku_id = skus[0]["id"] if skus else 0
    if sku_id:
        r = req("POST", "/api/v1/orders", {"skuId": sku_id}, token=token)
        order = r.get("data") or {}
        step("下单", r["code"] == 0 and order.get("id"), f"order={order.get('orderNo')}")
        r = req("POST", f"/api/v1/orders/{order['id']}/prepay", {"channel": "native"}, token=token)
        step("Native 通道支付（mock 直成功）", r["code"] == 0 and (r["data"] or {}).get("mockPaid"))
        r = req("GET", f"/api/v1/orders/{order['id']}", token=token)
        step("订单状态轮询", (r.get("data") or {}).get("status") == "paid")

    # 7. 订单状态过滤
    r = req("GET", "/api/v1/orders?page=1&status=paid", token=token)
    step("订单 status 过滤", r["code"] == 0 and (r["data"] or {}).get("total", 0) >= 1)
    r = req("GET", "/api/v1/orders?page=1&status=bogus", token=token)
    step("非法状态参数被拒", r["code"] != 0)

    # 8. 60s 重发限频
    r = req("POST", "/api/v1/auth/sms/send", {"phone": phone})
    step("短信 60s 重发限频", r["code"] != 0, r.get("msg", ""))

    print("\n==> 网页端冒烟全部通过")


if __name__ == "__main__":
    main()
