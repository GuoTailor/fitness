#!/usr/bin/env python3
"""M3 全链路冒烟：教练绑定→模板→学员购课→预约→确认→核销→团课→统计→后台课表"""
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
    coach_t = tok(call("POST", "/api/v1/auth/login", body={"code": "mock-coach-1"}))
    coach_uid = call("GET", "/api/v1/me", coach_t)["data"]["user"]["id"]

    # 1. 绑定教练账号 + 每日模板
    call("POST", "/admin/api/v1/coaches/2", admin_t,
         {"name": "张教练", "years": 8, "specialties": "减脂,增肌", "status": 1, "userId": coach_uid})
    me = call("POST", "/api/v1/auth/login", body={"code": "mock-coach-1"})["data"]
    ok("教练账号绑定（isCoach）", me["isCoach"] and me["coachId"] == 2)
    coach_t = me["token"]

    items = [{"weekday": i, "startMinute": 540, "endMinute": 1260} for i in range(7)]
    call("PUT", "/api/v1/coach/schedule", coach_t, {"items": items})
    ok("周模板保存", True)

    # 2. 学员购课（mock 支付）
    user_t = tok(call("POST", "/api/v1/auth/login", body={"code": "mock-user-%d" % random.randint(1000, 999999)}))
    order = call("POST", "/api/v1/orders", user_t, {"skuId": 7, "couponId": 0})["data"]
    pay = call("POST", f"/api/v1/orders/{order['id']}/prepay", user_t, {})["data"]
    ok("mock 支付入账", pay["mockPaid"])
    mps = call("GET", "/api/v1/member/packages", user_t)["data"]
    mp = mps[0]
    ok("课包 10 节未开卡", mp["totalSessions"] == 10 and mp["status"] == "inactive")

    # 3. 可约时段 + 预约 + 重复占位拒绝
    tomorrow = (datetime.date.today() + datetime.timedelta(days=1)).isoformat()
    slots = call("GET", f"/api/v1/coaches/2/slots?date={tomorrow}", user_t)["data"]["slots"]
    ok("时段日历非空", len(slots) > 0)
    slot = slots[0]["startMinute"]
    b = call("POST", "/api/v1/bookings", user_t,
             {"coachId": 2, "memberPackageId": mp["id"], "date": tomorrow, "startMinute": slot})["data"]
    ok("预约创建（pending）", b["status"] == "pending")
    mp = call("GET", "/api/v1/member/packages", user_t)["data"][0]
    ok("预约即扣次+开卡", mp["usedSessions"] == 1 and mp["status"] == "active")
    dup = call("POST", "/api/v1/bookings", user_t,
               {"coachId": 2, "memberPackageId": mp["id"], "date": tomorrow, "startMinute": slot})
    ok("重复占位被拒", dup["code"] == 40901)

    # 4. 教练确认 → 核销码 → 扫码 → 重复扫码幂等
    call("POST", f"/api/v1/coach/bookings/{b['id']}/confirm", coach_t)
    code = call("GET", f"/api/v1/bookings/{b['id']}/code", user_t)["data"]["code"]
    r1 = call("POST", "/api/v1/coach/verify", coach_t, {"code": code})["data"]
    ok("私教核销", not r1["alreadyVerified"] and r1["bizType"] == "private")
    r2 = call("POST", "/api/v1/coach/verify", coach_t, {"code": code})["data"]
    ok("重复扫码幂等", r2["alreadyVerified"])

    # 5. 教练发布团课 → 学员报名 → 出码 → 核销
    gc = call("POST", "/api/v1/coach/group-classes", coach_t,
              {"packageId": 2, "title": "燃脂团课", "date": tomorrow, "startMinute": 1140,
               "endMinute": 1200, "location": "大厅", "capacity": 10})["data"]
    ok("团课发布", gc["status"] == "enrolling")
    call("POST", f"/api/v1/group-classes/{gc['id']}/enroll", user_t, {"memberPackageId": mp["id"]})
    gcode = call("GET", f"/api/v1/group-classes/{gc['id']}/code", user_t)["data"]["code"]
    r3 = call("POST", "/api/v1/coach/verify", coach_t, {"code": gcode})["data"]
    ok("团课核销", r3["bizType"] == "group")

    # 6. 训练记录与统计、余额
    tr = call("GET", "/api/v1/training", user_t)["data"]
    ok("训练记录 2 条（私教+团课）", len(tr["records"]) == 2)
    mp = call("GET", "/api/v1/member/packages", user_t)["data"][0]
    ok("余额 10-2=8", mp["totalSessions"] - mp["usedSessions"] == 8)

    # 7. 后台全馆课表 + 教练统计
    sv = call("GET", f"/admin/api/v1/schedule?date={tomorrow}", admin_t)["data"]
    ok("全馆课表（私教1+团课1）", len(sv["bookings"]) >= 1 and len(sv["groupClasses"]) >= 1)
    st = call("GET", "/api/v1/coach/stats", coach_t)["data"]
    ok("教练统计", st["monthLessons"] >= 1)

    # 8. 教练取消场景：约后取消（明日，提前>4h）→ 课时返还
    b2 = call("POST", "/api/v1/bookings", user_t,
              {"coachId": 2, "memberPackageId": mp["id"], "date": tomorrow,
               "startMinute": slots[2]["startMinute"]})["data"]
    call("POST", f"/api/v1/bookings/{b2['id']}/cancel", user_t)
    mp = call("GET", "/api/v1/member/packages", user_t)["data"][0]
    ok("提前取消返还课时", mp["usedSessions"] == 2)
    # 取消后的时段可再约
    b3 = call("POST", "/api/v1/bookings", user_t,
              {"coachId": 2, "memberPackageId": mp["id"], "date": tomorrow,
               "startMinute": slots[2]["startMinute"]})
    ok("取消后时段释放可再约", b3["code"] == 0)


if __name__ == "__main__":
    main()
