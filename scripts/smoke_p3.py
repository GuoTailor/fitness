#!/usr/bin/env python3
"""P2 冒烟：老带新裂变（绑定/老客拒绝/首单双端发券/不重发）、打卡挑战（达标领取/重复拒绝）、
评价奖励券 + 续费提醒券（tasks/run 真库 E2E）、拼团（开团/参团/成团/入账/成团后拒绝）、
拼团活动层（限时覆盖/落地页/名额上限/下架回落）、
教练入驻（提交/重复拒绝/审核通过自动绑定 isCoach/驳回留痕）、logout、tasks/run 权限"""
import json
import random
import urllib.error
import urllib.parse
import urllib.request
from datetime import date, timedelta

B = "http://localhost:8080"
ADMIN = {"username": "admin", "password": "fitness123"}
COACH_ID = 2

# 最小合法 PNG（1x1 透明点）
PNG = bytes.fromhex(
    "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489"
    "0000000d49444154789c626001000000050001a5f645400000000049454e44ae426082")

RESULTS = []


def ok(label, cond):
    RESULTS.append((label, bool(cond)))
    print(("PASS  " if cond else "FAIL  ") + label)


def call(method, path, token=None, body=None):
    req = urllib.request.Request(B + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode() if body is not None else None
    with urllib.request.urlopen(req, data) as r:
        return json.loads(r.read())


def call_err(method, path, token=None, body=None):
    try:
        return 200, call(method, path, token, body)
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read())
        except Exception:
            return e.code, {}


def tok(code):
    return call("POST", "/api/v1/auth/login", body={"code": code})["data"]["token"]


def upload(token, filename="cert.png"):
    boundary = "----p3boundary"
    head = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; "
            f"filename=\"{filename}\"\r\nContent-Type: image/png\r\n\r\n").encode()
    payload = head + PNG + f"\r\n--{boundary}--\r\n".encode()
    req = urllib.request.Request(B + "/api/v1/uploads", data=payload, method="POST")
    req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
    req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["data"]["url"]


def coupon_count(token, tpl_id):
    items = call("GET", "/api/v1/coupons/mine?status=usable", token)["data"]["items"]
    return sum(1 for c in items if c["templateId"] == tpl_id)


def main():
    rnd = random.randint(100000, 999999)
    admin_t = tok_admin = None
    resp = call("POST", "/admin/api/v1/auth/login", body=ADMIN)
    admin_t = resp["data"]["token"]

    # ---- 0. 奖励券模板 + 配置 ----
    tpl1 = call("POST", "/admin/api/v1/coupons", admin_t, {
        "name": "P3奖励券", "type": "reduce", "threshold": 0, "amount": 3000,
        "packageId": 0, "totalCount": 0, "perUserLimit": 5, "validDays": 30})["data"]
    tpl2 = call("POST", "/admin/api/v1/coupons", admin_t, {
        "name": "P3续费券", "type": "reduce", "threshold": 0, "amount": 2000,
        "packageId": 0, "totalCount": 0, "perUserLimit": 5, "validDays": 30})["data"]
    for key, val in [
        ("member.warn_sessions", "10"),  # 放宽预警阈值让 D 的余课触发续费券
        ("marketing.referral_inviter_template", str(tpl1["id"])),
        ("marketing.referral_invitee_template", str(tpl1["id"])),
        ("marketing.checkin_target", "2"),
        ("marketing.checkin_reward_template", str(tpl1["id"])),
        ("marketing.renewal_reminder_template", str(tpl2["id"])),
        ("marketing.review_reward_template", str(tpl1["id"])),
    ]:
        call("POST", "/admin/api/v1/configs", admin_t, {"key": key, "value": val, "remark": "P3冒烟"})

    a_t = tok(f"mock-user-p3a-{rnd}")     # 邀请人
    b_t = tok(f"mock-user-p3b-{rnd}")     # 被邀请人
    d_t = tok(f"mock-user-p3d-{rnd}")     # 老客/打卡/续费/评价
    e_t = tok(f"mock-user-p3e-{rnd}")     # 入驻申请人
    f_t = tok(f"mock-user-p3f-{rnd}")     # 活动名额校验
    a_uid = call("GET", "/api/v1/me", a_t)["data"]["user"]["id"]

    # ---- 1. 老带新裂变 ----
    ok("绑定成功", call("POST", "/api/v1/referral/bind", b_t, {"inviter": a_uid})["data"]["bound"])
    st, dup = call_err("POST", "/api/v1/referral/bind", b_t, {"inviter": a_uid})
    ok("重复绑定拒绝", dup["code"] != 0 and "已绑定" in dup.get("msg", ""))
    st, selfb = call_err("POST", "/api/v1/referral/bind", a_t, {"inviter": a_uid})
    ok("自邀拒绝", selfb["code"] != 0)
    a_coupons_before = coupon_count(a_t, tpl1["id"])

    # B 首单实付 → 双方发券
    pkgs = call("GET", "/api/v1/packages", b_t)["data"]["list"]
    sku = call("GET", f"/api/v1/packages/{pkgs[0]['id']}", b_t)["data"]["skus"][0]
    order_b = call("POST", "/api/v1/orders", b_t, {"skuId": sku["id"], "couponId": 0})["data"]
    call("POST", f"/api/v1/orders/{order_b['id']}/prepay", b_t, {})
    b_mine = call("GET", "/api/v1/referral/mine", b_t)["data"]
    ok("被邀请人标记已奖励", b_mine["bound"] and b_mine["rewarded"])
    ok("邀请人得奖励券", coupon_count(a_t, tpl1["id"]) == a_coupons_before + 1)
    ok("被邀请人得奖励券", coupon_count(b_t, tpl1["id"]) == 1)
    # B 再买一单：不重发
    order_b2 = call("POST", "/api/v1/orders", b_t, {"skuId": sku["id"], "couponId": 0})["data"]
    call("POST", f"/api/v1/orders/{order_b2['id']}/prepay", b_t, {})
    ok("重复下单不重发", coupon_count(a_t, tpl1["id"]) == a_coupons_before + 1)

    # 老客（D 已购课后）绑定被拒
    d_pkgs = call("GET", "/api/v1/packages", d_t)["data"]["list"]
    d_sku = call("GET", f"/api/v1/packages/{d_pkgs[0]['id']}", d_t)["data"]["skus"][0]
    d_order = call("POST", "/api/v1/orders", d_t, {"skuId": d_sku["id"], "couponId": 0})["data"]
    call("POST", f"/api/v1/orders/{d_order['id']}/prepay", d_t, {})
    st, old = call_err("POST", "/api/v1/referral/bind", d_t, {"inviter": a_uid})
    ok("老客绑定拒绝", old["code"] != 0 and "不符合邀请条件" in old.get("msg", ""))

    # ---- 2. 打卡挑战：D 预约 2 次 → 核销 → 达标领取 ----
    coach_t = tok("mock-coach-1")
    tomorrow = (date.today() + timedelta(days=1)).isoformat()
    slots = call("GET", f"/api/v1/coaches/{COACH_ID}/slots?date={tomorrow}", d_t)["data"]["slots"]
    ok("可约时段非空", len(slots) >= 2)
    d_mp = call("GET", "/api/v1/member/packages", d_t)["data"][0]
    bookings = []
    for s in slots:
        if len(bookings) >= 2:
            break
        r = call_err("POST", "/api/v1/bookings", d_t,
                     {"coachId": COACH_ID, "memberPackageId": d_mp["id"],
                      "date": tomorrow, "startMinute": s["startMinute"]})
        if r[0] == 200:
            bookings.append(r[1]["data"])
    ok("预约 2 单成功", len(bookings) == 2)
    for b in bookings:
        call("POST", f"/api/v1/coach/bookings/{b['id']}/confirm", coach_t)
        code = call("GET", f"/api/v1/bookings/{b['id']}/code", d_t)["data"]["code"]
        call("POST", "/api/v1/coach/verify", coach_t, {"code": code})
    ch = call("GET", "/api/v1/checkin/challenge", d_t)["data"]
    ok("打卡进度达标", ch["enabled"] and ch["count"] == 2 and ch["claimable"])
    ok("达标领取成功", call("POST", "/api/v1/checkin/claim", d_t, {})["data"]["claimed"])
    st, again = call_err("POST", "/api/v1/checkin/claim", d_t, {})
    ok("重复领取拒绝", again["code"] != 0 and "已领取" in again.get("msg", ""))

    # ---- 3. 评价奖励券（自动过审即发）+ 续费券（tasks/run 真库 E2E） ----
    pend = call("GET", "/api/v1/reviews/reviewable", d_t)["data"]["items"]
    call("POST", "/api/v1/reviews", d_t,
         {"verificationId": pend[0]["verificationId"], "packageRating": 5, "coachRating": 5,
          "content": "P3冒烟评价"})
    ok("评价奖励券到账", coupon_count(d_t, tpl1["id"]) >= 1)
    d_coupons_before = coupon_count(d_t, tpl2["id"])
    call("POST", "/admin/api/v1/tasks/run", admin_t, {})
    ok("续费提醒券到账（预警任务）", coupon_count(d_t, tpl2["id"]) == d_coupons_before + 1)

    # ---- 4. 拼团 ----
    gp = call("POST", "/admin/api/v1/packages", admin_t, {
        "title": "P3冒烟拼团包", "category": "group", "status": 1, "difficulty": 2,
        "skus": [{"id": 0, "name": "4次拼团卡", "sessions": 4, "validDays": 60,
                  "price": 100000, "originalPrice": 100000, "groupPrice": 66000,
                  "groupCapacity": 2, "status": 1, "sort": 1}],
        "coachIds": [COACH_ID]})
    ok("拼团课包创建", gp["code"] == 0)
    kw = urllib.parse.quote("P3冒烟拼团包")
    gkw = [p for p in call("GET", f"/api/v1/packages?keyword={kw}", a_t)["data"]["list"]
           if p["title"] == "P3冒烟拼团包"][0]
    gsku = call("GET", f"/api/v1/packages/{gkw['id']}", a_t)["data"]["skus"][0]
    ok("SKU 拼团字段输出", gsku["groupPrice"] == 66000 and gsku["groupCapacity"] == 2)

    g_order_a = call("POST", "/api/v1/group-buys", a_t, {"skuId": gsku["id"]})["data"]
    ok("开团返回拼团订单", g_order_a["groupBuyId"] > 0 and g_order_a["amount"] == 66000)
    call("POST", f"/api/v1/orders/{g_order_a['id']}/prepay", a_t, {})
    g_order_b = call("POST", f"/api/v1/group-buys/{g_order_a['groupBuyId']}/join", b_t, {})["data"]
    call("POST", f"/api/v1/orders/{g_order_b['id']}/prepay", b_t, {})
    g_detail = call("GET", f"/api/v1/group-buys/{g_order_a['groupBuyId']}", a_t)["data"]
    ok("两人支付自动成团", g_detail["status"] == 2 and g_detail["paidCount"] == 2)
    st, late = call_err("POST", f"/api/v1/group-buys/{g_order_a['groupBuyId']}/join", e_t, {})
    ok("成团后参团拒绝", late["code"] != 0)
    a_mp = [m for m in call("GET", "/api/v1/member/packages", a_t)["data"] if m["skuId"] == gsku["id"]]
    ok("拼团课时入账", a_mp and a_mp[0]["totalSessions"] == 4)
    admin_gbs = call("GET", "/admin/api/v1/group-buys?status=2", admin_t)["data"]
    ok("后台拼团列表可见", any(g["id"] == g_order_a["groupBuyId"] for g in admin_gbs["list"]))

    # ---- 4b. 拼团活动层 ----
    from datetime import datetime
    now_s = datetime.now()
    act_body = {
        "title": "P3冒烟限时团", "skuId": gsku["id"], "groupPrice": 49000, "groupCapacity": 2,
        "startAt": (now_s - timedelta(minutes=2)).strftime("%Y-%m-%d %H:%M:%S"),
        "endAt": (now_s + timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
        "maxGroups": 1, "expireHours": 0, "status": 1}
    act_res = call("POST", "/admin/api/v1/group-buy-activities", admin_t, act_body)
    ok("活动创建成功", act_res["code"] == 0 and act_res["data"]["id"] > 0)
    act_id = act_res["data"]["id"]

    det = call("GET", f"/api/v1/packages/{gkw['id']}", a_t)["data"]
    det_sku = [s for s in det["skus"] if s["id"] == gsku["id"]][0]
    ok("课包详情活动覆盖", det_sku["groupPrice"] == 49000 and det_sku["groupActivityId"] == act_id
       and det_sku["groupActivityTitle"] == "P3冒烟限时团")

    act_order = call("POST", "/api/v1/group-buys", e_t, {"skuId": gsku["id"]})["data"]
    ok("活动价开团", act_order["amount"] == 49000 and act_order["groupBuyId"] > 0)

    landing = call("GET", f"/api/v1/group-buy-activities/{act_id}")["data"]
    ok("活动落地接口", landing["packageId"] == gkw["id"] and landing["skuId"] == gsku["id"]
       and landing["effectiveText"] == "进行中")

    st, full = call_err("POST", "/api/v1/group-buys", f_t, {"skuId": gsku["id"]})
    ok("名额满拒绝开团", full["code"] != 0 and "名额已满" in full.get("msg", ""))

    acts = call("GET", "/admin/api/v1/group-buy-activities", admin_t)["data"]["list"]
    act_row = [x for x in acts if x["id"] == act_id][0]
    ok("后台活动列表统计", act_row["openedCount"] == 1 and act_row["successCount"] == 0
       and act_row["effectiveText"] == "进行中")

    gbs_all = call("GET", f"/admin/api/v1/group-buys?status=1", admin_t)["data"]
    ok("拼团列表带活动标题", any(g["activityTitle"] == "P3冒烟限时团" for g in gbs_all["list"]))

    st, del_full = call_err("DELETE", f"/admin/api/v1/group-buy-activities/{act_id}", admin_t)
    ok("已开团活动禁删", del_full["code"] != 0 and "不能删除" in del_full.get("msg", ""))

    # 下架 → 课包详情回落 SKU 常开配置
    call("POST", f"/admin/api/v1/group-buy-activities/{act_id}", admin_t, {
        **act_body, "id": act_id,
        "startAt": act_row["startAt"], "endAt": act_row["endAt"], "status": 0})
    det2 = call("GET", f"/api/v1/packages/{gkw['id']}", a_t)["data"]
    det2_sku = [s for s in det2["skus"] if s["id"] == gsku["id"]][0]
    ok("下架后回落 SKU 配置", det2_sku["groupPrice"] == 66000 and det2_sku["groupActivityId"] == 0)

    # ---- 5. 教练入驻 ----
    cert = upload(e_t)
    ap = call("POST", "/api/v1/coach-applications", e_t, {
        "name": "入驻教练", "gender": 1, "years": 6, "specialties": "减脂,塑形",
        "intro": "P3 冒烟申请", "phone": "", "certImages": json.dumps([cert])})
    ok("入驻申请提交", ap["code"] == 0)
    st, dup_ap = call_err("POST", "/api/v1/coach-applications", e_t, {
        "name": "入驻教练", "years": 6, "certImages": json.dumps([cert])})
    ok("重复申请拒绝", dup_ap["code"] != 0 and "待审核" in dup_ap.get("msg", ""))
    ap_list = call("GET", "/admin/api/v1/coach-applications?status=0", admin_t)["data"]["list"]
    target = [x for x in ap_list if x["application"]["userId"] == ap["data"]["userId"]]
    ok("后台可见申请", len(target) == 1)
    call("POST", f"/admin/api/v1/coach-applications/{target[0]['application']['id']}/approve", admin_t, {})
    e_me = call("GET", "/api/v1/me", e_t)["data"]
    ok("通过后自动绑定 isCoach", e_me["isCoach"] and e_me["coachId"] > 0)
    # 驳回路径：D 提交再驳回
    cert2 = upload(d_t)
    ap2 = call("POST", "/api/v1/coach-applications", d_t, {
        "name": "驳回教练", "years": 3, "certImages": json.dumps([cert2])})["data"]
    call("POST", f"/admin/api/v1/coach-applications/{ap2['id']}/reject", admin_t,
         {"reason": "资质照片模糊"})
    d_mine = call("GET", "/api/v1/coach-applications/mine", d_t)["data"]
    ok("驳回留痕可见原因", d_mine["exists"] and d_mine["application"]["status"] == 2
       and d_mine["application"]["rejectReason"] == "资质照片模糊")
    ap3 = call("POST", "/api/v1/coach-applications", d_t, {
        "name": "驳回教练", "years": 3, "certImages": json.dumps([cert2])})
    ok("驳回后可重新申请", ap3["code"] == 0)

    # ---- 6. logout + tasks/run 权限 ----
    ok("学员登出", call("POST", "/api/v1/auth/logout", e_t, {})["data"]["logout"])
    ok("前台触发任务被拒", call_err("POST", "/admin/api/v1/tasks/run", a_t, {})[1]["code"] != 0)
    ok("老板手动触发任务", call("POST", "/admin/api/v1/tasks/run", admin_t, {})["data"]["ran"])

    # ---- 清理营销配置（避免污染日常开发）----
    for key in ["marketing.referral_inviter_template", "marketing.referral_invitee_template",
                "marketing.checkin_target", "marketing.checkin_reward_template",
                "marketing.renewal_reminder_template", "marketing.review_reward_template",
                "member.warn_sessions"]:
        call("POST", "/admin/api/v1/configs", admin_t, {"key": key, "value": "", "remark": "P3冒烟清理"})

    failed = [label for label, passed in RESULTS if not passed]
    print(f"\n{len(RESULTS) - len(failed)}/{len(RESULTS)} passed")
    if failed:
        raise SystemExit("FAILED: " + "; ".join(failed))


if __name__ == "__main__":
    main()
    print("smoke_p3 done")
