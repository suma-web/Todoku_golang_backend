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
   * VUごとにユーザーを割り当てる
   */
  const user =
    users[(__VU - 1) % users.length];

  try {
    /*
     * 1. Todokuへアクセス
     */
    await page.goto(BASE_URL);

    /*
     * 2. 認証判定待ち
     */
    await page.waitForTimeout(1000);

    console.log(
      `VU=${__VU} before login: ${page.url()}`
    );

    /*
     * 3. ログイン
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
     * 4. ログイン処理待ち
     */
    await page.waitForTimeout(2000);

    console.log(
      `VU=${__VU} after login: ${page.url()}`
    );

    /*
     * 5. ログイン成功確認
     */
    check(page, {
      "login succeeded": (p) =>
        !p.url().includes("/login"),
    });

    /*
     * 実際のユーザーが画面を見る時間を再現
     */
    sleep(
      Math.random() * 3 + 2
    );

  } finally {
    await page.close();
  }
}