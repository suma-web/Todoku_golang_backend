// ブラウザログイン試験: Chromiumで100 VUが各1回実行する（最大2分）。
import { browser } from "k6/browser";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";

const BASE_URL =
  __ENV.BASE_URL ||
  "https://main.da845b239xog4.amplifyapp.com";

const users = new SharedArray("users", function () {
  return JSON.parse(open("./users.json"));
});

export const options = {
  scenarios: {
    browser_test: {
      executor: "per-vu-iterations",

      // 100 VUで並行実行する。厳密な同時送信は保証しない。
      vus: 100,
      iterations: 1,

      maxDuration: "2m",

      options: {
        browser: {
          type: "chromium",
        },
      },
    },
  },

  thresholds: {
    checks: ["rate>0.99"],

    browser_web_vital_lcp: [
      "p(95)<2500",
    ],

    browser_web_vital_fcp: [
      "p(95)<1800",
    ],
  },
};

export default async function () {
  const page = await browser.newPage();

  /*
   * VUごとにユーザーを割り当てる（ユーザー数が不足すると同じアカウントを再利用）
   */
  const user =
    users[(__VU - 1) % users.length];

  try {
    /*
     * 1. Todokuへアクセス
     */
    await page.goto(BASE_URL);

    /*
     * 2. 認証判定を想定して1秒固定待機（判定完了を直接待つ処理ではない）
     */
    await page.waitForTimeout(1000);

    console.log(
      `VU=${__VU} before login: ${page.url()}`
    );

    /*
     * 3. URLに/loginが含まれる場合のみログインフォームを送信
     */
    if (page.url().includes("/login")) {
      await page
        .locator('input[type="email"]')
        .fill(user.email);

      await page
        .locator('input[type="password"]')
        .fill(user.password);

      await page
        .locator('button[type="submit"]')
        .click();
    }

    /*
     * 4. ログイン処理を想定して2秒固定待機
     */
    await page.waitForTimeout(2000);

    console.log(
      `VU=${__VU} after login: ${page.url()}`
    );

    /*
     * 5. URLに/loginが含まれないことを確認（認証済みデータの表示は確認しない）
     */
    check(page, {
      "login succeeded": (p) =>
        !p.url().includes("/login"),
    });

    /*
     * 画面を見る時間として2〜5秒待機
     */
    sleep(
      Math.random() * 3 + 2
    );

  } finally {
    await page.close();
  }
}