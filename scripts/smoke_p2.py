#!/usr/bin/env python3
"""P1 二期冒烟：活动价 / 赠课券（入账+退款）/ 教练请假 / 团课候补自动递补 /
课后反馈 / 体重+周报 / 问答区 / 资料编辑 / 评分筛选 / 审计日志（评价奖励与续费券为单测覆盖）"""
import datetime
import json
import random
import urllib.error
import urllib.request

B = "http://localhost:8080"
ADMIN = {"username": "admin", "password": "fitness123"}


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
    try:
        r = call(method, path, token, body)
        return 200, r
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"raw": raw.decode("utf-8", "ignore")}


RESULTS = {"pass": 0, "fail": 0}


def ok(label, cond):
    print(("PASS  " if cond else "FAIL  ") + label)
    if cond:
        RESULTS["pass"] += 1
    else:
        RESULTS["fail"] += 1


def fmt_dt(offset_hours):
    return (datetime.datetime.now() + datetime.timedelta(hours=offset_hours)).strftime("%Y-%m-%d %H:%M:%S")


def main():
    admin_t = tok(call("POST", "/admin/api/v1/auth/login", body=ADMIN))
    coach_t = tok(call("POST", "/api/v1/auth/login", body={"code": "mock-coach-1"}))
    coach_uid = call("GET", "/api/v1/me", coach_t)["data"]["user"]["id"]
    call("POST", "/admin/api/v1/coaches/2", admin_t,
         {"name": "张教练", "years": 8, "specialties": "减脂,增肌", "status": 1, "userId": coach_uid})
    # 教练身份 DB 实时判定，绑定后原 token 即可进教练端（少一次登录，避开 IP 限流）

    def new_user():
        return tok(call("POST", "/api/v1/auth/login",
                        body={"code": "mock-user-%d" % random.randint(1000, 999999)}))

    user_a = new_user()  # 活动课包：赠课券+预约核销反馈+退款
    user_b = new_user()  # 团课候补发起者
    user_c = new_user()  # 团课候补中签者
    tomorrow = (datetime.date.today() + datetime.timedelta(days=1)).isoformat()

    # ---- ① 限时活动价：建课包（活动价生效）+ 未生效窗口对照 ----
    call("POST", "/admin/api/v1/packages", admin_t, {
        "title": "P2冒烟活动课", "category": "private", "status": 1, "difficulty": 2,
        "skus": [
            {"id": 0, "name": "10次卡", "sessions": 10, "validDays": 365, "price": 200000,
             "originalPrice": 200000, "promoPrice": 150000,
             "promoStart": fmt_dt(-1), "promoEnd": fmt_dt(48), "status": 1, "sort": 1},
            {"id": 0, "name": "未来活动卡", "sessions": 5, "validDays": 365, "price": 100000,
             "originalPrice": 100000, "promoPrice": 60000,
             "promoStart": fmt_dt(24), "promoEnd": fmt_dt(48), "status": 1, "sort": 2},
        ],
        "coachIds": [2],
    })
    import urllib.parse
    kw = urllib.parse.quote("P2冒烟活动课")
    promo_pkg = [p for p in call("GET", f"/api/v1/packages?keyword={kw}", user_a)["data"]["list"]
                 if p["title"] == "P2冒烟活动课"][0]
    pdetail = call("GET", f"/api/v1/packages/{promo_pkg['id']}", user_a)["data"]
    sku_active = [s for s in pdetail["skus"] if s["name"] == "10次卡"][0]
    sku_future = [s for s in pdetail["skus"] if s["name"] == "未来活动卡"][0]
    ok("活动价生效标记", sku_active["promoActive"] is True)
    ok("未到窗口不生效", sku_future["promoActive"] is False)
    o_promo = call("POST", "/api/v1/orders", user_a,
                   {"skuId": sku_future["id"], "couponId": 0})["data"]
    ok("未生效按原价下单", o_promo["amount"] == 100000 and o_promo["promoUsed"] == 0)

    # ---- ② 赠课券：领取 → 可用券（不降价）→ 叠加活动价下单 → 入账赠课时 ----
    gtpl = call("POST", "/admin/api/v1/coupons", admin_t, {
        "name": "P2冒烟赠课券", "type": "gift", "amount": 2, "packageId": 0,
        "totalCount": 10, "perUserLimit": 1, "validDays": 30,
    })["data"]
    ok("老板建赠课券", gtpl["id"] > 0)
    call("POST", f"/api/v1/coupons/{gtpl['id']}/claim", user_a, {})
    usable = [c for c in call("GET", f"/api/v1/coupons/usable?skuId={sku_active['id']}",
                              user_a)["data"]["items"] if c["templateId"] == gtpl["id"]]
    ok("可用券含赠课券（benefit=0）", len(usable) == 1 and usable[0]["giftSessions"] == 2
       and usable[0]["finalPay"] == 200000)
    gift_order = call("POST", "/api/v1/orders", user_a,
                      {"skuId": sku_active["id"], "couponId": usable[0]["couponId"]})["data"]
    ok("赠课券叠加活动价", gift_order["amount"] == 150000 and gift_order["promoUsed"] == 1
       and gift_order["giftSessions"] == 2 and gift_order["couponAmount"] == 0)
    call("POST", f"/api/v1/orders/{gift_order['id']}/prepay", user_a, {})
    mp_a = [m for m in call("GET", "/api/v1/member/packages", user_a)["data"]
            if m["packageId"] == promo_pkg["id"]][0]
    ok("入账含赠课（10+2）", mp_a["totalSessions"] == 12 and mp_a["giftSessions"] == 2)

    # ---- ③ 教练请假：屏蔽时段 + 拒单 + 撤销恢复 ----
    items = [{"weekday": i, "startMinute": 540, "endMinute": 1260} for i in range(7)]
    call("PUT", "/api/v1/coach/schedule", coach_t, {"items": items})
    # 选当前最后一个空闲格做请假窗口（避开历史冒烟数据占用的时段）
    slots0 = call("GET", f"/api/v1/coaches/2/slots?date={tomorrow}", user_a)["data"]["slots"]
    lv_start = slots0[-1]["startMinute"]
    leave = call("POST", "/api/v1/coach/leaves", coach_t,
                 {"date": tomorrow, "startMinute": lv_start, "endMinute": lv_start + 60,
                  "reason": "P2冒烟"})["data"]
    ok("登记请假", leave["leave"]["id"] > 0)
    slots = call("GET", f"/api/v1/coaches/2/slots?date={tomorrow}", user_a)["data"]["slots"]
    in_leave = [s for s in slots if lv_start <= s["startMinute"] < lv_start + 60]
    ok("请假窗口时段被屏蔽", len(in_leave) == 0 and len(slots) > 0)
    st, rej = call_err("POST", "/api/v1/bookings", user_a,
                       {"coachId": 2, "memberPackageId": mp_a["id"], "date": tomorrow,
                        "startMinute": lv_start})
    ok("请假时段预约被拒", "请假" in rej.get("msg", ""))
    call("DELETE", f"/api/v1/coach/leaves/{leave['leave']['id']}", coach_t)
    slots2 = call("GET", f"/api/v1/coaches/2/slots?date={tomorrow}", user_a)["data"]["slots"]
    ok("撤销请假后恢复", any(s["startMinute"] == lv_start for s in slots2))

    # ---- ④ 预约→核销→课后反馈 ----
    slot = slots2[0]["startMinute"]
    booking = call("POST", "/api/v1/bookings", user_a,
                   {"coachId": 2, "memberPackageId": mp_a["id"], "date": tomorrow,
                    "startMinute": slot})["data"]
    call("POST", f"/api/v1/coach/bookings/{booking['id']}/confirm", coach_t)
    code = call("GET", f"/api/v1/bookings/{booking['id']}/code", user_a)["data"]["code"]
    call("POST", "/api/v1/coach/verify", coach_t, {"code": code})
    fb = call("POST", "/api/v1/coach/feedback", coach_t,
              {"verificationId": 0, "content": "x"})  # 先验证 404/错误路径
    ok("无效核销单写反馈被拒", fb["code"] != 0)
    ver = call("GET", "/api/v1/reviews/reviewable", user_a)["data"]["items"]
    vid = ver[0]["verificationId"]
    call("POST", "/api/v1/coach/feedback", coach_t,
         {"verificationId": vid, "content": "深蹲动作标准，下次加强核心训练（P2冒烟）"})
    mine_fb = call("GET", "/api/v1/feedback/mine", user_a)["data"]
    ok("学员可见课后反馈", len(mine_fb) >= 1 and "P2冒烟" in mine_fb[0]["feedback"]["content"])
    bd = call("GET", f"/api/v1/bookings/{booking['id']}", user_a)["data"]
    ok("预约详情带反馈", bd["feedback"] and "核心" in bd["feedback"]["content"])

    # ---- ⑤ 体重 + 训练周报 ----
    today = datetime.date.today().isoformat()
    yesterday = (datetime.date.today() - datetime.timedelta(days=1)).isoformat()
    call("POST", "/api/v1/body-metrics", user_a, {"date": yesterday, "weightG": 70500})
    call("POST", "/api/v1/body-metrics", user_a, {"date": today, "weightG": 70000})
    metrics = call("GET", "/api/v1/body-metrics?limit=30", user_a)["data"]
    ok("体重列表升序 2 条", len(metrics) == 2 and metrics[0]["recordDate"][:10] <= metrics[1]["recordDate"][:10])
    rep = call("GET", "/api/v1/training/report", user_a)["data"]
    expect_delta = 0.0 if rep["weekStart"] == today else -0.5
    ok("周报聚合与体重差", rep["current"]["sessions"] >= 1
       and abs(rep["weight"]["deltaKg"] - expect_delta) < 0.001)

    # ---- ⑥ 退款（已开卡折算：赠课剔除+封顶实付）→ 赠课券返还 ----
    rf = call("POST", f"/api/v1/orders/{gift_order['id']}/refund", user_a,
              {"reason": "P2冒烟"})["data"]
    done = call("POST", f"/admin/api/v1/refunds/{rf['id']}/approve", admin_t)["data"]
    ok("退款封顶实付（赠课剔除后折算>实付）", done["amount"] == 150000)
    usable_after = call("GET", f"/api/v1/coupons/mine?status=usable", user_a)["data"]["items"]
    ok("退款后赠课券返还", any(c["couponId"] == usable[0]["couponId"] for c in usable_after))

    # ---- ⑦ 团课候补：满员登记 → 退出自动递补（扣次）→ 取消候补 → 整课取消作废 ----
    call("POST", "/admin/api/v1/packages", admin_t, {
        "title": "P2冒烟团课包", "category": "group", "status": 1, "difficulty": 2,
        "skus": [{"id": 0, "name": "10次卡", "sessions": 10, "validDays": 365,
                  "price": 100000, "originalPrice": 100000, "status": 1, "sort": 1}],
        "coachIds": [2],
    })
    gkw = urllib.parse.quote("P2冒烟团课包")
    group_pkg = [p_ for p_ in call("GET", f"/api/v1/packages?keyword={gkw}", user_b)["data"]["list"]
                 if p_["title"] == "P2冒烟团课包"][0]
    gdetail = call("GET", f"/api/v1/packages/{group_pkg['id']}", user_b)["data"]
    gsku = gdetail["skus"][0]
    for t in (user_b, user_c):
        o = call("POST", "/api/v1/orders", t, {"skuId": gsku["id"], "couponId": 0})["data"]
        call("POST", f"/api/v1/orders/{o['id']}/prepay", t, {})
    mp_b = [m for m in call("GET", "/api/v1/member/packages", user_b)["data"]
            if m["packageId"] == group_pkg["id"]][0]
    mp_c = [m for m in call("GET", "/api/v1/member/packages", user_c)["data"]
            if m["packageId"] == group_pkg["id"]][0]

    gc1 = call("POST", "/api/v1/coach/group-classes", coach_t,
               {"packageId": group_pkg["id"], "title": "P2候补课", "date": tomorrow,
                "startMinute": 1140, "endMinute": 1200, "location": "大厅", "capacity": 1})["data"]
    call("POST", f"/api/v1/group-classes/{gc1['id']}/enroll", user_b, {"memberPackageId": mp_b["id"]})
    wl = call("POST", f"/api/v1/group-classes/{gc1['id']}/waitlist", user_c,
              {"memberPackageId": mp_c["id"]})["data"]
    ok("候补登记（不扣次）", wl["status"] == "waiting")
    ok("已报名不可候补", call("POST", f"/api/v1/group-classes/{gc1['id']}/waitlist", user_b,
                             {"memberPackageId": mp_b["id"]})["code"] != 0)
    det_c = call("GET", f"/api/v1/group-classes/{gc1['id']}", user_c)["data"]
    ok("候补标记", det_c["waitlisted"] is True)
    call("POST", f"/api/v1/group-classes/{gc1['id']}/quit", user_b)
    det_c2 = call("GET", f"/api/v1/group-classes/{gc1['id']}", user_c)["data"]
    mp_c2 = [m for m in call("GET", "/api/v1/member/packages", user_c)["data"]
             if m["packageId"] == group_pkg["id"]][0]
    ok("退出自动递补（中签+扣次）", det_c2["myStatus"] == "enrolled"
       and det_c2["waitlisted"] is False and mp_c2["usedSessions"] == 1)

    # 主动取消候补
    gc2 = call("POST", "/api/v1/coach/group-classes", coach_t,
               {"packageId": group_pkg["id"], "title": "P2候补课2", "date": tomorrow,
                "startMinute": 900, "endMinute": 960, "location": "大厅", "capacity": 1})["data"]
    call("POST", f"/api/v1/group-classes/{gc2['id']}/enroll", user_b, {"memberPackageId": mp_b["id"]})
    call("POST", f"/api/v1/group-classes/{gc2['id']}/waitlist", user_c, {"memberPackageId": mp_c2["id"]})
    call("POST", f"/api/v1/group-classes/{gc2['id']}/waitlist/cancel", user_c)
    det = call("GET", f"/api/v1/group-classes/{gc2['id']}", user_c)["data"]
    ok("主动取消候补", det["waitlisted"] is False)
    # 整课取消 → 候补作废（user_b 重新候补 gc2 后取消整课）
    call("POST", f"/api/v1/group-classes/{gc2['id']}/quit", user_b)
    call("POST", f"/api/v1/group-classes/{gc2['id']}/waitlist", user_b, {"memberPackageId": mp_b["id"]})
    call("POST", f"/api/v1/coach/group-classes/{gc2['id']}/cancel", coach_t, {"reason": "P2冒烟"})
    det_b = call("GET", f"/api/v1/group-classes/{gc2['id']}", user_b)["data"]
    ok("整课取消候补作废", det_b["waitlisted"] is False)

    # ---- ⑧ 问答区：提问（匿名）/ 陌生人不可回 / 教练回复 / 后台隐藏+置顶 ----
    qa = call("POST", "/api/v1/qa/questions", user_b,
              {"packageId": promo_pkg["id"], "content": "零基础可以上吗？需要自带装备吗？", "anonymous": True})
    ok("提问过短被拒", call("POST", "/api/v1/qa/questions", user_b,
                            {"packageId": promo_pkg["id"], "content": "短", "anonymous": False})["code"] != 0)
    qid = qa["data"]["id"]
    ok("提问成功（先发后审可见）", qa["code"] == 0)
    stranger_t = new_user()
    ok("陌生人不可回复", call("POST", f"/api/v1/qa/questions/{qid}/answers", stranger_t,
                             {"content": "我不知道"})["code"] != 0)
    ans = call("POST", f"/api/v1/coach/qa/{qid}/answers", coach_t,
               {"content": "零基础完全没问题，穿运动鞋来就行（P2冒烟）"})
    ok("教练回复", ans["code"] == 0)
    qlist = call("GET", f"/api/v1/packages/{promo_pkg['id']}/qa", user_b)["data"]["list"]
    q0 = [q for q in qlist if q["id"] == qid][0]
    ok("匿名展示+教练优先", q0["asker"] == "匿名学员"
       and q0["answers"][0]["isCoach"] == 1)
    call("POST", f"/admin/api/v1/qa/questions/{qid}/status", admin_t, {"status": "hidden"})
    qlist2 = call("GET", f"/api/v1/packages/{promo_pkg['id']}/qa", user_b)["data"]["list"]
    ok("后台隐藏问题", all(q["id"] != qid for q in qlist2))
    call("POST", f"/admin/api/v1/qa/questions/{qid}/status", admin_t, {"status": "visible"})
    call("POST", f"/admin/api/v1/qa/questions/{qid}/pin", admin_t, {"pinned": True})
    qlist3 = call("GET", f"/api/v1/packages/{promo_pkg['id']}/qa", user_b)["data"]["list"]
    ok("置顶排第一", qlist3[0]["id"] == qid and qlist3[0]["pinned"] == 1)
    mine_q = call("GET", "/api/v1/qa/mine", user_b)["data"]
    ok("我的提问（本人实名可见）", any(q["id"] == qid and q["asker"] != "匿名学员" for q in mine_q))

    # ---- ⑨ 资料编辑 + 评分筛选 ----
    up = call("PUT", "/api/v1/me", user_b,
              {"nickname": "P2冒烟学员", "avatarUrl": "/uploads/images/202610/x.png"})
    ok("资料保存", up["data"]["nickname"] == "P2冒烟学员")
    ok("外链头像被拒", call("PUT", "/api/v1/me", user_b,
                            {"nickname": "x", "avatarUrl": "https://evil.example/a.png"})["code"] != 0)
    all_coaches = call("GET", "/api/v1/coaches", user_b)["data"]["list"]
    high = call("GET", "/api/v1/coaches?minRating=4.9", user_b)["data"]["list"]
    ok("评分筛选生效", len(high) <= len(all_coaches))

    # ---- ⑩ 审计日志：老板可查、前台 403 ----
    logs = call("GET", "/admin/api/v1/audit-logs", admin_t)["data"]
    actions = [r["action"] for r in logs["list"]]
    ok("审计含退款审批", "refund.approve" in actions)
    ok("审计含建券", "coupon.save" in actions)
    front_t = tok(call("POST", "/admin/api/v1/auth/login",
                       body={"username": "front", "password": "fitness123"}))
    st, body = call_err("GET", "/admin/api/v1/audit-logs", front_t)
    ok("前台不可查审计(403)", st == 403 and body["code"] == 40301)

    print(f"\nsmoke_p2: {RESULTS['pass']} pass / {RESULTS['fail']} fail")
    if RESULTS["fail"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
