import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { randomBytes } from "node:crypto";

const root = new URL("../", import.meta.url).pathname;
const base = process.env.VIDEOHUB_API ?? "http://127.0.0.1:8080";
const output = root + ".run/moderation-verification";
mkdirSync(output, { recursive: true });
const results = [];
const check = (name) => {
  results.push({ name, result: "PASS" });
  console.log("PASS " + name);
};
const docker = (args) =>
  execFileSync("docker", ["compose", ...args], {
    cwd: root,
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  });
async function post(path, body = {}, token = "", expected = 200) {
  const res = await fetch(base + path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: "Bearer " + token } : {}),
    },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  assert.equal(res.status, expected, `${path}: ${JSON.stringify(data)}`);
  return data;
}
let accounts;
const credentials = output + "/accounts.json";
if (existsSync(credentials))
  accounts = JSON.parse(readFileSync(credentials, "utf8"));
else {
  const suffix = Date.now().toString().slice(-8);
  accounts = {
    admin: {
      account_name: "91" + suffix,
      username: "审核验证管理员",
      password: "Vh" + randomBytes(10).toString("hex"),
    },
    author: {
      account_name: "92" + suffix,
      username: "投稿验证作者",
      password: "Vh" + randomBytes(10).toString("hex"),
    },
    other: {
      account_name: "93" + suffix,
      username: "权限验证用户",
      password: "Vh" + randomBytes(10).toString("hex"),
    },
  };
  for (const account of Object.values(accounts))
    await post("/account/register", account);
  writeFileSync(credentials, JSON.stringify(accounts, null, 2), {
    mode: 0o600,
  });
}
if (process.env.STAFF_ACCOUNT && process.env.STAFF_PASSWORD) {
  accounts.staff = {account_name: process.env.STAFF_ACCOUNT, password: process.env.STAFF_PASSWORD};
  writeFileSync(credentials, JSON.stringify(accounts, null, 2), {mode: 0o600});
}
const author = (await post("/account/login", accounts.author)).token;
const other = (await post("/account/login", accounts.other)).token;
const communityAdmin = (await post("/account/login", accounts.admin)).token;
const admin = (await post("/admin/login", accounts.staff ?? accounts.admin)).token;
await post("/admin/videos", {}, "", 401);
await post("/admin/videos", {}, author, 401);
await post("/admin/videos", {}, communityAdmin, 401);
await post("/admin/login", accounts.other, "", 401);
await post("/account/me", {}, communityAdmin);
check("管理员独立登录、普通用户与社区令牌隔离");

docker([
  "exec",
  "-T",
  "worker",
  "ffmpeg",
  "-hide_banner",
  "-loglevel",
  "error",
  "-f",
  "lavfi",
  "-i",
  "testsrc2=size=640x360:rate=24",
  "-f",
  "lavfi",
  "-i",
  "sine=frequency=440:sample_rate=44100",
  "-t",
  "3",
  "-c:v",
  "libx264",
  "-pix_fmt",
  "yuv420p",
  "-c:a",
  "aac",
  "-y",
  "/tmp/review-fixture.mp4",
]);
docker([
  "exec",
  "-T",
  "worker",
  "ffmpeg",
  "-hide_banner",
  "-loglevel",
  "error",
  "-f",
  "lavfi",
  "-i",
  "color=c=0x20785e:s=640x360",
  "-frames:v",
  "1",
  "-update",
  "1",
  "-y",
  "/tmp/review-cover.jpg",
]);
docker(["cp", "worker:/tmp/review-fixture.mp4", output + "/fixture.mp4"]);
docker(["cp", "worker:/tmp/review-cover.jpg", output + "/cover.jpg"]);
async function upload(path, file, type) {
  const form = new FormData();
  form.append(
    "file",
    new Blob([readFileSync(output + "/" + file)], { type }),
    file,
  );
  const res = await fetch(base + path, {
    method: "POST",
    headers: { Authorization: "Bearer " + author },
    body: form,
  });
  const data = await res.json();
  assert.equal(res.status, 200, JSON.stringify(data));
  return data;
}
async function submission(title) {
  const cover = await upload("/video/uploadCover", "cover.jpg", "image/jpeg");
  const raw = await upload("/video/uploadVideo", "fixture.mp4", "video/mp4");
  assert.equal(
    (await fetch(base + raw.play_url)).status,
    404,
    "raw upload leaked",
  );
  assert.equal(
    (await fetch(base + cover.cover_url)).status,
    404,
    "unsubmitted cover leaked",
  );
  const v = await post(
    "/video/publish",
    {
      title,
      description: "独立审核功能完整验证样片",
      play_object_key: raw.object_key,
      cover_object_key: cover.object_key,
    },
    author,
  );
  for (let i = 0; i < 90; i++) {
    const current = await post("/video/processingStatus", { id: v.id }, author);
    if (current.status === "pending_review") return v.id;
    assert.notEqual(current.status, "published", "media worker auto-published");
    assert.notEqual(current.status, "failed", current.error);
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error("media processing timed out");
}
const approved = await submission("审核验证：通过后公开");
const rejected = await submission("审核验证：驳回并反馈原因");
const pending = await submission("审核验证：待审核示例");
check("真实上传、OSS/本地存储、FFmpeg 转码后停在待审状态");
await post("/video/getDetail", { id: approved }, "", 404);
await post("/video/submission", { id: approved }, other, 404);
await post("/like/like", { video_id: approved }, other, 400);
const detail = await post("/admin/detail", { id: approved }, admin);
assert.equal(
  (await fetch(base + "/static/" + detail.video.play_object_key)).status,
  404,
);
const preview = detail.video.play_url.replace(/^\/api/, "");
const playback = await fetch(base + preview, {
  headers: { Range: "bytes=0-99" },
});
assert.equal(playback.status, 206);
assert.equal((await playback.arrayBuffer()).byteLength, 100);
await post("/video/selectCover", { id: approved, index: 0 }, author, 400);
check("待审视频详情、互动、文件直链隔离，管理员可按 Range 预览");
const decisions = await Promise.all(
  [0, 1].map(() =>
    fetch(base + "/admin/decide", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + admin,
      },
      body: JSON.stringify({ id: approved, decision: "approve" }),
    }),
  ),
);
assert.deepEqual(decisions.map((r) => r.status).sort(), [200, 409]);
const published = await post("/video/getDetail", { id: approved });
assert.equal(published.status, "published");
assert(published.published_at);
const publishedMedia = await fetch(
  published.play_url.startsWith("http")
    ? published.play_url
    : base + published.play_url,
  { headers: { Range: "bytes=0-99" } },
);
assert.equal(publishedMedia.status, 206);
const audit = await post("/admin/detail", { id: approved }, admin);
assert.equal(audit.reviews.length, 1);
check("并发审批只成功一次，审核日志与公开播放正确");
await post("/admin/decide", { id: rejected, decision: "reject" }, admin, 400);
await post(
  "/admin/decide",
  {
    id: rejected,
    decision: "reject",
    reason: "封面与视频内容不符，请修改后重新投稿。",
  },
  admin,
);
const rejectedSubmission = await post(
  "/video/submission",
  { id: rejected },
  author,
);
assert.equal(rejectedSubmission.status, "rejected");
assert(rejectedSubmission.review_reason.includes("封面"));
await post("/video/getDetail", { id: rejected }, "", 404);
await post("/admin/decide", { id: rejected, decision: "approve" }, admin, 409);
let notifications;
for (let i = 0; i < 15; i++) {
  notifications = await post("/notification/list", { type: "review" }, author);
  if (notifications.notifications.some((n) => n.target_id === rejected)) break;
  await new Promise((r) => setTimeout(r, 1000));
}
assert(notifications.notifications.some((n) => n.target_id === approved));
assert(notifications.notifications.some((n) => n.target_id === rejected));
check("驳回必填原因、作者可见、审核结果通知正确");
let feed;
for (let i = 0; i < 15; i++) {
  feed = await post("/feed/listLatest", { limit: 50, latest_time: 0 });
  if (feed.video_list.some((v) => v.id === approved)) break;
  await new Promise((r) => setTimeout(r, 1000));
}
assert(feed.video_list.some((v) => v.id === approved));
assert(!feed.video_list.some((v) => [rejected, pending].includes(v.id)));
check("Outbox 与 RabbitMQ 发布事件进入社区信息流，待审和驳回保持隐藏");
const fileID = randomBytes(16).toString("hex");
const source = readFileSync(output + "/fixture.mp4");
const middle = Math.ceil(source.length / 2);
for (let index = 0; index < 2; index++) {
  const response = await fetch(base + "/video/uploadChunk", {
    method: "POST", headers: { Authorization: "Bearer " + author, "X-File-ID": fileID, "X-Chunk-Index": String(index), "X-Total-Chunks": "2" },
    body: source.subarray(index * middle, Math.min((index + 1) * middle, source.length)),
  });
  assert.equal(response.status, 200);
}
assert.equal((await fetch(base + "/static/chunks/" + fileID + "/0")).status, 404);
const merged = await post("/video/mergeChunks", { file_id: fileID, file_ext: ".mp4" }, author);
const withdrawn = await post("/video/publish", { title: "审核验证：处理中撤回", play_object_key: merged.object_key }, author);
await post("/video/delete", { id: withdrawn.id }, other, 400);
await post("/video/delete", { id: withdrawn.id }, author);
await post("/admin/decide", { id: withdrawn.id, decision: "approve" }, admin, 409);
await post("/video/submission", { id: withdrawn.id }, author, 404);
check("分片上传合并、处理中撤回、禁止越权删除和审批已撤回视频");
const badForm = new FormData();
badForm.append("file", new Blob(["not a real video"], { type: "video/mp4" }), "invalid.mp4");
const badUpload = await fetch(base + "/video/uploadVideo", { method: "POST", headers: { Authorization: "Bearer " + author }, body: badForm });
assert.equal(badUpload.status, 200);
const badFile = await badUpload.json();
const failed = await post("/video/publish", { title: "审核验证：媒体处理失败", play_object_key: badFile.object_key }, author);
let failure;
for (let i = 0; i < 30; i++) { failure = await post("/video/processingStatus", { id: failed.id }, author); if (failure.status === "failed") break; await new Promise(r => setTimeout(r, 1000)); }
assert.equal(failure.status, "failed");
await post("/admin/decide", { id: failed.id, decision: "approve" }, admin, 409);
await post("/video/delete", { id: failed.id }, author);
const page1 = await post("/admin/videos", { status: "published", limit: 1 }, admin);
assert(page1.items.length <= 1 && page1.total >= 1);
check("无效媒体进入处理失败状态且不能审批，审核列表分页正确");
await post("/admin/logout", {}, admin);
assert.equal((await fetch(base + preview)).status, 403);
check("管理员退出立即撤销媒体预览权限");
writeFileSync(
  output + "/state.json",
  JSON.stringify(
    { approved, rejected, pending, author, other, communityAdmin },
    null,
    2,
  ),
  { mode: 0o600 },
);
writeFileSync(
  output + "/api-results.json",
  JSON.stringify({ time: new Date().toISOString(), results }, null, 2),
);
console.log(
  "Verification saved to .run/moderation-verification; credentials are in accounts.json (0600).",
);
