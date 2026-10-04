// 認証セッション試験: 10 VUで3分間、Cookieを保持して5回操作ごとにログアウトする。
import http from "k6/http";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";

const API_URL =
  __ENV.API_URL ||
  "https://to-607423c6bdff4743a6450c75a0c81ce0.ecs.ap-northeast-1.on.aws";

const users = new SharedArray("users", () => {
  return JSON.parse(open("./users.json"));
});

let loggedIn = false;
let actionCount = 0;

export const options = {
  vus: 10,
  duration: "3m",

  // iterationをまたいでもCookieを保持
  noCookiesReset: true,

  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const user = users[(__VU - 1) % users.length];

  // -------------------------
  // 1. 未ログイン時のみログイン（反復間で認証状態とCookieを保持）
  // -------------------------
  if (!loggedIn) {
    const loginRes = http.post(
      `${API_URL}/api/login`,
      JSON.stringify({
        email: user.email,
        password: user.password,
      }),
      {
        headers: {
          "Content-Type": "application/json",
        },
        tags: {
          name: "POST /api/login",
        },
      }
    );

    check(loginRes, {
      "login status is 200": (r) => r.status === 200,
    });

    if (loginRes.status !== 200) {
      console.error(
        `LOGIN_FAILED VU=${__VU} status=${loginRes.status} body=${loginRes.body}`
      );

      sleep(3);
      return;
    }

    loggedIn = true;
    actionCount = 0;

    // 認証Cookieをログに出力するため、出力結果の共有時には取り扱いに注意。
    const cookies = http
      .cookieJar()
      .cookiesForURL(API_URL);

    console.log(
      `LOGIN_OK VU=${__VU} cookies=${JSON.stringify(cookies)}`
    );
  }

  // -------------------------
  // 2. ユーザー情報の200応答を確認。失敗時は次の反復で再ログイン
  // -------------------------
  const cookiesBeforeMe = http
    .cookieJar()
    .cookiesForURL(API_URL);

  const meRes = http.get(
    `${API_URL}/api/me`,
    {
      tags: {
        name: "GET /api/me",
      },
    }
  );

  check(meRes, {
    "me status is 200": (r) => r.status === 200,
  });

  if (meRes.status !== 200) {
    console.error(
      `ME_FAILED VU=${__VU} ` +
      `status=${meRes.status} ` +
      `cookies=${JSON.stringify(cookiesBeforeMe)} ` +
      `body=${meRes.body}`
    );

    loggedIn = false;
    actionCount = 0;

    sleep(3);
    return;
  }

  // -------------------------
  // 3. タイムラインの200応答を確認（失敗でも操作回数を加算）
  // -------------------------
  const timelineRes = http.get(
    `${API_URL}/api/timeline`,
    {
      tags: {
        name: "GET /api/timeline",
      },
    }
  );

  check(timelineRes, {
    "timeline status is 200": (r) =>
      r.status === 200,
  });

  actionCount++;

  console.log(
    `VU=${__VU} action=${actionCount} ` +
    `me=${meRes.status} timeline=${timelineRes.status}`
  );

  sleep(2);

  // -------------------------
  // 4. タイムライン取得を5回試行したらログアウトし、200または204を確認
  // -------------------------
  if (actionCount < 5) {
    return;
  }

  const logoutRes = http.post(
    `${API_URL}/api/logout`,
    null,
    {
      tags: {
        name: "POST /api/logout",
      },
    }
  );

  check(logoutRes, {
    "logout succeeded": (r) =>
      r.status === 200 || r.status === 204,
  });

  console.log(
    `LOGOUT VU=${__VU} ` +
    `status=${logoutRes.status} ` +
    `error=${logoutRes.error || ""} ` +
    `body=${logoutRes.body}`
  );

  // -------------------------
  // 5. ログアウト後の401を確認。
  // 現在は401をexpectedStatusesに指定していないため、HTTP失敗率にも算入される。
  // -------------------------
  const afterLogoutMe = http.get(
    `${API_URL}/api/me`,
    {
      tags: {
        name: "GET /api/me after logout",
      },
    }
  );

  check(afterLogoutMe, {
    "me is 401 after logout": (r) =>
      r.status === 401,
  });

  console.log(
    `AFTER_LOGOUT VU=${__VU} me=${afterLogoutMe.status}`
  );

  loggedIn = false;
  actionCount = 0;

  sleep(5);
}