import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chromium } = require(
  process.env.PLAYWRIGHT_MODULE ??
    "/tmp/videohub-browser/node_modules/playwright",
);
const root = new URL("../", import.meta.url).pathname,
  output = root + ".run/moderation-verification";
const accounts = JSON.parse(readFileSync(output + "/accounts.json", "utf8")),
  state = JSON.parse(readFileSync(output + "/state.json", "utf8"));
const staff = accounts.staff ?? accounts.admin;
const adminURL = process.env.REVIEW_URL ?? "http://127.0.0.1:5176";
const results = [],
  errors = [];
const pass = (name) => {
  console.log("PASS " + name);
  results.push({ name, result: "PASS" });
};
const browser = await chromium.launch({
  headless: true,
  args: ["--no-sandbox"],
});
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
});
const page = await context.newPage();
page.on("pageerror", (e) => errors.push(e.message));
page.setDefaultTimeout(15000);
try {
  await page.goto(adminURL + "/reviews");
  await page.waitForURL("**/login");
  await page
    .getByLabel("账号", { exact: true })
    .fill(accounts.other.account_name);
  await page.getByLabel("密码", { exact: true }).fill(accounts.other.password);
  await page.getByRole("button", { name: "登录审核中心", exact: true }).click();
  await page.getByRole("alert").waitFor();
  assert((await page.getByRole("alert").innerText()).includes("无审核权限"));
  await page
    .getByLabel("账号", { exact: true })
    .fill(staff.account_name);
  await page.getByLabel("密码", { exact: true }).fill(staff.password);
  await page.getByRole("button", { name: "登录审核中心", exact: true }).click();
  await page.waitForURL("**/reviews");
  await page.locator(".queue-item").first().waitFor();
  pass("独立审核站登录、普通账号拒绝、管理员登录后进入工作台");
  await page.goto(adminURL + "/reviews/" + state.pending);
  await page.waitForFunction(
    () => document.querySelector("video")?.readyState >= 2,
  );
  await page.locator("video").evaluate(async (v) => {
    v.muted = true;
    await v.play();
  });
  await page.waitForFunction(
    () => document.querySelector("video")?.currentTime > 0.15,
  );
  const pixels = await page.locator("video").evaluate((v) => {
    const canvas = document.createElement("canvas");
    canvas.width = 32;
    canvas.height = 18;
    const c = canvas.getContext("2d");
    c.drawImage(v, 0, 0, 32, 18);
    const a = c.getImageData(0, 0, 32, 18).data;
    return new Set(Array.from(a)).size;
  });
  assert(pixels > 20, "video frames blank");
  await page.locator("video").evaluate((v) => v.pause());
  await page.screenshot({
    path: output + "/admin-desktop.png",
    fullPage: true,
  });
  assert(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "desktop overflow",
  );
  pass("真实视频预览、播放帧与桌面布局");
  await page.getByRole("button", { name: "驳回", exact: true }).click();
  assert(
    await page
      .getByRole("button", { name: "确认驳回", exact: true })
      .isDisabled(),
  );
  await page
    .getByLabel("驳回原因", { exact: true })
    .fill("浏览器验证：请更换与内容一致的封面。");
  await page.getByRole("button", { name: "确认驳回", exact: true }).click();
  await page.locator(".history").getByText("已驳回", { exact: true }).waitFor();
  pass("浏览器驳回操作、原因校验和审核历史");
  await page.getByLabel("搜索标题或作者").fill("不存在的审核标题");
  await page.getByRole("button", { name: "搜索", exact: true }).click();
  await page.getByText("暂无待审核视频", { exact: true }).waitFor();
  await page.getByLabel("搜索标题或作者").fill("");
  await page.getByRole("button", { name: "搜索", exact: true }).click();
  pass("审核列表搜索与空结果");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(adminURL + "/reviews/" + state.approved);
  await page.locator(".history").waitFor();
  await page.screenshot({ path: output + "/admin-mobile.png", fullPage: true });
  assert(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "mobile overflow",
  );
  pass("手机宽度下审核详情无横向溢出");
  for (const [name, port, width, height] of [
    ["desktop", 5173, 1440, 1000],
    ["mobile", 5174, 390, 844],
  ]) {
    const userContext = await browser.newContext({
      viewport: { width, height },
    });
    await userContext.addInitScript(
      (token) => localStorage.setItem("jwt_token", token),
      state.author,
    );
    const authorPage = await userContext.newPage();
    authorPage.on("pageerror", (e) => errors.push(e.message));
    await authorPage.goto(
      "http://127.0.0.1:" + port + "/submissions/" + state.rejected,
    );
    await authorPage
      .getByText("审核意见：封面与视频内容不符，请修改后重新投稿。", {
        exact: true,
      })
      .waitFor();
    await authorPage.waitForFunction(
      () => document.querySelector("video")?.readyState >= 2,
    );
    await authorPage.screenshot({
      path: output + "/author-" + name + ".png",
      fullPage: true,
    });
    assert(
      await authorPage.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      name + " author overflow",
    );
    await authorPage.goto("http://127.0.0.1:" + port + "/messages");
    await authorPage
      .getByText("你的作品未通过审核：封面与视频内容不符，请修改后重新投稿。", {
        exact: true,
      })
      .first()
      .waitFor();
    await authorPage
      .getByText("你的作品未通过审核：封面与视频内容不符，请修改后重新投稿。", {
        exact: true,
      })
      .first()
      .click();
    await authorPage.waitForURL("**/submissions/" + state.rejected);
    pass(name + " 作者投稿状态、私有播放与审核通知跳转");
    await authorPage.goto(
      "http://127.0.0.1:" + port + (name === "desktop" ? "/video" : "/publish"),
    );
    await authorPage
      .locator("input[type=file]")
      .nth(0)
      .setInputFiles(output + "/fixture.mp4");
    await authorPage
      .locator("input[type=file]")
      .nth(1)
      .setInputFiles(output + "/cover.jpg");
    await authorPage
      .getByPlaceholder(
        name === "desktop"
          ? "填写作品标题，让更多人发现你的视频"
          : "添加作品标题",
      )
      .fill("浏览器投稿验证 " + name);
    const submitted = authorPage.waitForResponse(
      (r) =>
        r.url().endsWith("/api/video/publish") &&
        r.request().method() === "POST",
    );
    await authorPage
      .getByRole("button", {
        name: name === "desktop" ? "发布视频" : "发布",
        exact: true,
      })
      .click();
    const created = await (await submitted).json();
    if (name === "desktop")
      await authorPage.getByRole("link", { name: "查看审核进度" }).click();
    await authorPage.waitForURL("**/submissions/" + created.id);
    await authorPage
      .locator(".detail-heading")
      .getByText("待审核", { exact: true })
      .waitFor({ timeout: 60000 });
    pass(name + " 浏览器选择文件、真实上传并进入待审投稿详情");
    if (name === "desktop") {
      await page.setViewportSize({ width: 1440, height: 1000 });
      await page.goto(adminURL + "/reviews/" + created.id);
      await page
        .getByRole("button", { name: "通过并发布", exact: true })
        .click();
      await page.getByRole("button", { name: "确认发布", exact: true }).click();
      await page
        .locator(".history")
        .getByText("审核通过", { exact: true })
        .waitFor();
      await authorPage
        .locator(".detail-heading")
        .getByText("已发布", { exact: true })
        .waitFor({ timeout: 30000 });
      pass("浏览器通过并发布，作者页面自动更新审核状态");
    }
    await userContext.close();
  }
  await page.getByRole("button", { name: "退出登录", exact: true }).click();
  await page.waitForURL("**/login");
  await page.goto(adminURL + "/reviews");
  await page.waitForURL("**/login");
  assert.deepEqual(errors, []);
  pass("退出登录保护和浏览器运行时无异常");
  writeFileSync(
    output + "/browser-results.json",
    JSON.stringify({ time: new Date().toISOString(), results }, null, 2),
  );
} finally {
  await browser.close();
}
