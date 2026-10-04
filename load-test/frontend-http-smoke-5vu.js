// フロントHTTP疎通試験: 5 VUで30秒間トップページをGETする。画面描画やログインは行わない。
import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL =
  __ENV.BASE_URL ||
  "https://main.da845b239xog4.amplifyapp.com";

export const options = {
  vus: 5,
  duration: "30s",

  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<1000"],
  },
};

export default function () {
  // HTMLのHTTP応答のみ測定する。JavaScript実行や追加リソースの取得は行わない。
  const res = http.get(BASE_URL);

  check(res, {
    "status is 200": (r) => r.status === 200,
    "response time < 1s": (r) => r.timings.duration < 1000,
  });

  // 各反復の末尾で1秒待機する。
  sleep(1);
}