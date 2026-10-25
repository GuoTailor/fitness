#!/usr/bin/env python3
"""P1 营销一期冒烟：上传→静态回取 / 协议公开接口 / 后台建券（前台403）/ 领券中心 /
券后下单→mock支付→退款→券返还 / 新人价两单 / 手机号查重 / 新人礼包自动发券"""
import json
import random
import urllib.error
import urllib.parse
import urllib.request

B = "http://localhost:8080"
ADMIN = {"username": "admin", "password": "fitness123"}

# 最小合法 PNG（1x1 透明点）
PNG = bytes.fromhex(
    "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489"
    "0000000d49444154789c626001000000050001a5f645400000000049454e44ae426082")


def call(method, path, token=None, body=None):
    req = urllib.request.Request(B + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    with urllib.request.urlopen(req, data) as r:
        return json.loads(r.read())


def tok(resp):
    return resp["data"]["token"]


def call_err(method, path, token=None, body=None):
    """期望失败的调用：HTTPError 时解析响应体，返回 (status, body)"""
    try:
        r = call(method, path, token, body)
        return 200, r
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"raw": raw.decode("utf-8", "ignore")}


def upload(token, filename, content):
    boundary = "----smokeboundary"
    head = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; "
            f"filename=\"{filename}\"\r\nContent-Type: image/png\r\n\r\n").encode()
    payload = head + content + f"\r\n--{boundary}--\r\n".encode()
    req = urllib.request.Request(B + "/api/v1/uploads", data=payload, method="POST")
    req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
    req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())


def main():
    ok = lambda label, cond: print(("PASS  " if cond else "FAIL  ") + label)

    admin_t = tok(call("POST", "/admin/api/v1/auth/login", body=ADMIN))
    user_t = tok(call("POST", "/api/v1/auth/login",
                      body={"code": "mock-user-%d" % random.randint(1000, 999999)}))
    phone = "139%08d" % random.randint(0, 99999999)

    # ---- ① 上传：合法 PNG → URL；静态回取；白名单外扩展名拒绝 ----
    up = upload(user_t, "a.png", PNG)["data"]
    ok("上传成功返回 URL", up["url"].startswith("http://localhost:8080/uploads/images/"))
    with urllib.request.urlopen(up["url"]) as r:
        back = r.read()
    ok("静态回取内容一致", back == PNG and len(back) > 50)
    st, bad = call_err("POST", "/api/v1/uploads", user_t)  # 缺 file 字段
    ok("缺文件字段被拒", bad["code"] != 0)
    # 非图片扩展名：业务错误码在 HTTP 200 里返回
    try:
        r2 = upload(user_t, "evil.txt", b"not an image")
        ok("非图片扩展名被拒", r2["code"] != 0)
    except urllib.error.HTTPError as e:
        ok("非图片扩展名被拒", json.loads(e.read())["code"] != 0)

    # ---- ② 协议公开接口（无登录）----
    privacy = call("GET", "/api/v1/agreement?type=privacy")
    agreement = call("GET", "/api/v1/agreement?type=user")
    ok("协议接口公开可读", privacy["code"] == 0 and agreement["data"]["title"] == "用户协议")

    # ---- ③ 后台建券：老板可建、前台 403 ----
    tpl = call("POST", "/admin/api/v1/coupons", admin_t, {
        "name": "P1冒烟满减券", "type": "reduce", "threshold": 50000, "amount": 5000,
        "packageId": 0, "totalCount": 10, "perUserLimit": 1, "validDays": 30,
    })["data"]
    ok("老板建券", tpl["id"] > 0 and tpl["status"] == 1)
    try:
        front_t = tok(call("POST", "/admin/api/v1/auth/login",
                           body={"username": "front", "password": "fitness123"}))
        st, body = call_err("POST", "/admin/api/v1/coupons", front_t, {
            "name": "x", "type": "reduce", "amount": 100, "validDays": 30})
        ok("前台不可建券(403)", st == 403 and body["code"] == 40301)
    except urllib.error.HTTPError:
        ok("前台不可建券(403)", False)

    # ---- ④ 领券中心：领取成功/重复限领/我的券包 ----
    center = call("GET", "/api/v1/coupons/center", user_t)["data"]["items"]
    mine_tpl = [c for c in center if c["templateId"] == tpl["id"]]
    ok("领券中心可见", len(mine_tpl) == 1 and mine_tpl[0]["claimable"])
    uc = call("POST", f"/api/v1/coupons/{tpl['id']}/claim", user_t, {})["data"]
    ok("领取成功", uc["status"] == "usable" and uc["expireAt"])
    st, again = call_err("POST", f"/api/v1/coupons/{tpl['id']}/claim", user_t, {})
    ok("重复领取被限领拒绝", st == 200 and again["code"] != 0)

    # ---- ⑤ 券后下单 → mock 支付 → 退款 → 券返还 ----
    pkgs = call("GET", "/api/v1/packages", user_t)["data"]["list"]
    pkg = pkgs[0]
    detail = call("GET", f"/api/v1/packages/{pkg['id']}", user_t)["data"]
    sku = detail["skus"][0]
    usable = [c for c in call("GET", f"/api/v1/coupons/usable?skuId={sku['id']}", user_t)["data"]["items"]
              if c["templateId"] == tpl["id"]]
    ok("可用券列表含新券", len(usable) == 1 and usable[0]["benefit"] == 5000)
    expect_pay = usable[0]["finalPay"]
    order = call("POST", "/api/v1/orders", user_t,
                 {"skuId": sku["id"], "couponId": usable[0]["couponId"]})["data"]
    ok("券后下单金额正确", order["amount"] == expect_pay and order["couponAmount"] == 5000)
    pay = call("POST", f"/api/v1/orders/{order['id']}/prepay", user_t, {})["data"]
    ok("mock 支付", pay["mockPaid"])
    rf = call("POST", f"/api/v1/orders/{order['id']}/refund", user_t, {"reason": "P1冒烟"})["data"]
    done = call("POST", f"/admin/api/v1/refunds/{rf['id']}/approve", admin_t)["data"]
    ok("退款金额=券后实付", done["amount"] == expect_pay and done["status"] == "refunded")
    mine = call("GET", "/api/v1/coupons/mine?status=usable", user_t)["data"]["items"]
    ok("退款后券返还 usable", any(c["couponId"] == usable[0]["couponId"] for c in mine))

    # ---- ⑥ 新人价：建新人 SKU → 绑手机号 → 首单新人价、二单原价 ----
    call("POST", "/admin/api/v1/packages", admin_t, {
        "title": "P1冒烟新人体验课", "category": "trial", "status": 1, "difficulty": 1,
        "skus": [{"id": 0, "name": "体验1次", "sessions": 1, "validDays": 30,
                  "price": 100000, "originalPrice": 100000, "newcomerPrice": 66000,
                  "status": 1, "sort": 1}],
        "coachIds": [2],
    })
    kw = urllib.parse.quote("P1冒烟新人体验课")
    np = [p for p in call("GET", f"/api/v1/packages?keyword={kw}", user_t)["data"]["list"]
          if p["title"] == "P1冒烟新人体验课"][0]
    ndetail = call("GET", f"/api/v1/packages/{np['id']}", user_t)["data"]
    nsku = ndetail["skus"][0]
    ok("未绑手机号不可享新人价", not nsku["newcomerEligible"])
    call("POST", "/api/v1/auth/phone", user_t, {"code": phone})
    ndetail = call("GET", f"/api/v1/packages/{np['id']}", user_t)["data"]
    ok("绑手机号后可享新人价", ndetail["skus"][0]["newcomerEligible"])
    o1 = call("POST", "/api/v1/orders", user_t, {"skuId": nsku["id"], "couponId": 0})["data"]
    ok("首单新人价", o1["amount"] == 66000 and o1["newcomerUsed"] == 1)
    o2 = call("POST", "/api/v1/orders", user_t, {"skuId": nsku["id"], "couponId": 0})["data"]
    ok("二单原价（限购一次）", o2["amount"] == 100000 and o2["newcomerUsed"] == 0)

    # ---- ⑦ 手机号查重（唯一索引语义）----
    other_t = tok(call("POST", "/api/v1/auth/login",
                       body={"code": "mock-user-%d" % random.randint(1000, 999999)}))
    st, dup = call_err("POST", "/api/v1/auth/phone", other_t, {"code": phone})
    ok("手机号重复绑定被拒", dup["code"] != 0 and "已绑定" in dup.get("msg", ""))

    # ---- ⑧ 后台定向发券 ----
    uid = call("GET", "/api/v1/me", other_t)["data"]["user"]["id"]
    call("POST", f"/admin/api/v1/coupons/{tpl['id']}/grant", admin_t, {"userId": uid})
    other_mine = call("GET", "/api/v1/coupons/mine?status=usable", other_t)["data"]["items"]
    ok("定向发放到位", any(c["templateId"] == tpl["id"] for c in other_mine))

    # ---- ⑨ 新人礼包：配置后新登录用户自动发券 ----
    call("POST", "/admin/api/v1/configs", admin_t,
         {"key": "marketing.newcomer_gift_templates", "value": str(tpl["id"]), "remark": "P1冒烟"})
    fresh_t = tok(call("POST", "/api/v1/auth/login",
                       body={"code": "mock-user-%d" % random.randint(1000, 999999)}))
    fresh_mine = call("GET", "/api/v1/coupons/mine?status=usable", fresh_t)["data"]["items"]
    ok("新用户自动领新人礼包", any(c["templateId"] == tpl["id"] for c in fresh_mine))
    call("POST", "/admin/api/v1/configs", admin_t,
         {"key": "marketing.newcomer_gift_templates", "value": "", "remark": "P1冒烟"})


if __name__ == "__main__":
    main()
    print("smoke_p1 done")
