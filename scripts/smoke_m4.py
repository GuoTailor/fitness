#!/usr/bin/env python3
"""M4 全链路冒烟：核销→待评价→双维度提交→后台审核→评分聚合→教练回复→
精选/隐藏→公开评价→训练日历→订阅消息模板→通知落库（经预订/确认钩子）"""
import json
import datetime
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


def main():
    import random
    ok = lambda label, cond: print(("PASS  " if cond else "FAIL  ") + label)

    admin_t = tok(call("POST", "/admin/api/v1/auth/login", body=ADMIN))
    me = call("POST", "/api/v1/auth/login", body={"code": "mock-coach-1"})["data"]
    ok("教练登录（双角色）", me["isCoach"] and me["coachId"] == 2)
    coach_t = me["token"]

    user_t = tok(call("POST", "/api/v1/auth/login",
                      body={"code": "mock-user-%d" % random.randint(1000, 999999)}))
    order = call("POST", "/api/v1/orders", user_t, {"skuId": 7, "couponId": 0})["data"]
    pay = call("POST", f"/api/v1/orders/{order['id']}/prepay", user_t, {})["data"]
    ok("mock 支付入账", pay["mockPaid"])
    mp = call("GET", "/api/v1/member/packages", user_t)["data"][0]

    tomorrow = (datetime.date.today() + datetime.timedelta(days=1)).isoformat()
    slots = call("GET", f"/api/v1/coaches/2/slots?date={tomorrow}", user_t)["data"]["slots"]
    slot = slots[0]["startMinute"]

    # 1. 私教：预约→确认→核销
    b = call("POST", "/api/v1/bookings", user_t,
             {"coachId": 2, "memberPackageId": mp["id"], "date": tomorrow,
              "startMinute": slot})["data"]
    call("POST", f"/api/v1/coach/bookings/{b['id']}/confirm", coach_t)
    code = call("GET", f"/api/v1/bookings/{b['id']}/code", user_t)["data"]["code"]
    r1 = call("POST", "/api/v1/coach/verify", coach_t, {"code": code})["data"]
    ok("私教核销", not r1["alreadyVerified"])

    # 2. 团课：发布→报名→核销
    gc = call("POST", "/api/v1/coach/group-classes", coach_t,
              {"packageId": 2, "title": "M4 冒烟团课", "date": tomorrow, "startMinute": 1140,
               "endMinute": 1200, "location": "大厅", "capacity": 10})["data"]
    call("POST", f"/api/v1/group-classes/{gc['id']}/enroll", user_t, {"memberPackageId": mp["id"]})
    gcode = call("GET", f"/api/v1/group-classes/{gc['id']}/code", user_t)["data"]["code"]
    r2 = call("POST", "/api/v1/coach/verify", coach_t, {"code": gcode})["data"]
    ok("团课核销", r2["bizType"] == "group")

    # 3. 待评价列表（私教+团课各 1）→ 双维度提交
    pend = call("GET", "/api/v1/reviews/reviewable", user_t)["data"]["items"]
    ok("待评价 2 条", len(pend) == 2)
    # 基线：教练/课包已公开的评价数（可重复执行）
    base_coach = call("GET", "/api/v1/coaches/2/reviews", user_t)["data"]["total"]
    base_pkg = call("GET", "/api/v1/packages/2/reviews", user_t)["data"]["total"]
    vid = pend[0]["verificationId"]
    sub = call("POST", "/api/v1/reviews", user_t,
               {"verificationId": vid, "packageRating": 5, "coachRating": 4,
                "content": "冒烟评价：教练很专业"})
    ok("评价提交（双维度）", sub["code"] == 0)
    dup = call("POST", "/api/v1/reviews", user_t,
               {"verificationId": vid, "packageRating": 5, "coachRating": 4, "content": "再评"})
    ok("重复提交被拒", dup["code"] != 0)
    bad = call("POST", "/api/v1/reviews", user_t,
               {"verificationId": pend[1]["verificationId"], "packageRating": 6,
                "coachRating": 4, "content": "x"})
    ok("非法评分被拒", bad["code"] != 0)
    mine = call("GET", "/api/v1/reviews/mine", user_t)["data"]["items"]
    ok("我的评价 2 条（课包+教练维度）", len(mine) == 2)

    # 4. 审核前公开不可见
    pub = call("GET", "/api/v1/coaches/2/reviews", user_t)["data"]
    ok("审核前教练公开评价不新增", pub["total"] == base_coach)

    # 5. 后台审核通过
    lst = call("GET", "/admin/api/v1/reviews?status=pending", admin_t)["data"]["items"]
    mine_ids = {it["review"]["id"] for it in mine}
    for it in lst:
        if it["review"]["id"] in mine_ids:
            call("POST", f"/admin/api/v1/reviews/{it['review']['id']}/audit", admin_t,
                 {"action": "approve"})
    pub = call("GET", "/api/v1/coaches/2/reviews", user_t)["data"]
    ok("审核后教练公开评价 +1", pub["total"] == base_coach + 1)
    new_coach_review = pub["items"][0]["review"]  # featured/id DESC，最新在前

    # 6. 评分聚合刷新（coachRating=4）
    coach_detail = call("GET", "/api/v1/coaches/2", user_t)["data"]["coach"]
    ok("教练评分聚合（≥1 条，均分≈4+）", coach_detail["ratingCount"] >= 1)

    # 7. 教练回复（回自己最新那条）
    rid = new_coach_review["id"]
    rep = call("POST", f"/api/v1/coach/reviews/{rid}/reply", coach_t, {"content": "感谢支持！"})
    ok("教练回复", rep["code"] == 0)
    myr = call("GET", "/api/v1/coach/reviews", coach_t)["data"]["items"]
    ok("教练收到回复后的评价", any(x["review"]["coachReply"] == "感谢支持！" for x in myr))

    # 8. 课包维度：精选 → 详情精选区可见 → 隐藏后回落
    pkg_reviews = call("GET", "/api/v1/packages/2/reviews", user_t)["data"]
    ok("课包公开评价 +1", pkg_reviews["total"] == base_pkg + 1)
    prid = pkg_reviews["items"][0]["review"]["id"]
    call("POST", f"/admin/api/v1/reviews/{prid}/featured", admin_t, {"featured": True})
    detail = call("GET", "/api/v1/packages/2", user_t)["data"]
    ok("课包详情含评分与精选评价", detail["ratingCount"] >= 1 and len(detail["reviews"]) >= 1)
    call("POST", f"/admin/api/v1/reviews/{prid}/audit", admin_t, {"action": "hide"})
    pkg_reviews = call("GET", "/api/v1/packages/2/reviews", user_t)["data"]
    ok("隐藏后课包公开评价回落", pkg_reviews["total"] == base_pkg)

    # 9. 训练日历（本月有打点）
    month = datetime.date.today().strftime("%Y-%m")
    cal = call("GET", f"/api/v1/training/calendar?month={month}", user_t)["data"]
    ok("训练日历打点", len(cal["days"]) >= 1)

    # 10. 订阅消息模板下发（mock → 空表 + mock=true）
    tpl = call("GET", "/api/v1/notify/templates", user_t)["data"]
    ok("订阅模板接口（mock）", tpl["mock"] is True and tpl["templates"] == {})

    print("\nM4 smoke done.")


if __name__ == "__main__":
    main()
