import http from "k6/http";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";

const API_URL =
  __ENV.API_URL ||
  "https://to-607423c6bdff4743a6450c75a0c81ce0.ecs.ap-northeast-1.on.aws";

const users = new SharedArray("users", () => {
  return JSON.parse(open("./users.json"));
});

// VUごとに状態保持
let loggedIn = false;
let actionCount = 0;

const MAX_ACTIONS_PER_SESSION = 5;

// emergency-spike-720.js

export const options = {
  noCookiesReset: true,

  scenarios: {
    emergency_spike: {
      executor: "ramping-vus",
      startVUs: 0,

      // 通常利用 645 VU
      stages: [
        { duration: "2m", target: 100 },
        { duration: "3m", target: 300 },
        { duration: "5m", target: 300 },
        { duration: "5m", target: 500 },
        { duration: "5m", target: 500 },
        { duration: "5m", target: 645 },
        { duration: "5m", target: 645 },
        { duration: "3m", target: 300 },
        { duration: "2m", target: 100 },
        { duration: "1m", target: 0 },
      ],

      gracefulRampDown: "30s",
    },
  },

  thresholds: {
    checks: ["rate>0.99"],
    http_req_failed: ["rate<0.01"],

    http_req_duration: ["p(95)<1000", "p(99)<2000"],

    "http_req_duration{name:POST /api/login}": ["p(95)<1500"],

    "http_req_duration{name:GET /api/me}": ["p(95)<1000"],

    "http_req_duration{name:GET /api/timeline}": ["p(95)<1500"],

    "http_req_duration{name:POST /api/logout}": ["p(95)<500"],
  },
};

export default function () {
  const user = users[(__VU - 1) % users.length];

  // -------------------------
  // 1. Login
  // -------------------------
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
    },
  );

  const loginSuccess = check(loginRes, {
    "login status is 200": (r) => r.status === 200,
  });

  if (!loginSuccess) {
    console.error(
      `LOGIN_FAILED VU=${__VU} status=${loginRes.status} error=${loginRes.error || ""}`,
    );

    sleep(5);
    return;
  }

  // -------------------------
  // 2. /api/me
  // -------------------------
  const meRes = http.get(`${API_URL}/api/me`, {
    tags: {
      name: "GET /api/me",
    },
  });

  const meSuccess = check(meRes, {
    "me status is 200": (r) => r.status === 200,
  });

  if (!meSuccess) {
    if (meRes.status === 401) {
      loggedIn = false;
      actionCount = 0;
    }

    console.error(`ME_FAILED VU=${__VU} status=${meRes.status}`);

    sleep(randomSleep(3, 5));
    return;
  }

  sleep(randomSleep(1, 3));

  // -------------------------
  // 3. Timeline
  // -------------------------
  const timelineRes = http.get(`${API_URL}/api/timeline`, {
    tags: {
      name: "GET /api/timeline",
    },
  });

  const timelineSuccess = check(timelineRes, {
    "timeline status is 200": (r) => r.status === 200,
  });

  if (!timelineSuccess) {
    console.error(`TIMELINE_FAILED VU=${__VU} status=${timelineRes.status}`);
  }

  actionCount++;

  // 実際に連絡を見る時間
  sleep(randomSleep(8, 20));

  // -------------------------
  // 4. セッション継続
  // -------------------------
  if (actionCount < MAX_ACTIONS_PER_SESSION) {
    return;
  }

  // -------------------------
  // 5. Logout
  // -------------------------
  const logoutRes = http.post(`${API_URL}/api/logout`, null, {
    tags: {
      name: "POST /api/logout",
    },
  });

  check(logoutRes, {
    "logout succeeded": (r) => r.status === 200 || r.status === 204,
  });

  // -------------------------
  // 6. Logout後の認証確認
  // -------------------------
  const afterLogoutMe = http.get(`${API_URL}/api/me`, {
    tags: {
      name: "GET /api/me after logout",
    },

    // 401を意図した正常レスポンスとして扱う
    responseCallback: http.expectedStatuses(401),
  });

  check(afterLogoutMe, {
    "me is 401 after logout": (r) => r.status === 401,
  });

  loggedIn = false;
  actionCount = 0;

  // 一旦アプリを離れる
  sleep(randomSleep(20, 60));
}

function randomSleep(min, max) {
  return Math.random() * (max - min) + min;
}
