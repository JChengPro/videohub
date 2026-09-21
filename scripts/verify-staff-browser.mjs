import assert from 'node:assert/strict';
import { readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { randomBytes } from 'node:crypto';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE ?? '/tmp/videohub-browser/node_modules/playwright');
const output = new URL('../.run/moderation-verification/', import.meta.url).pathname;
const accounts = JSON.parse(readFileSync(output+'accounts.json', 'utf8'));
const staff = accounts.staff ?? accounts.admin;
const state = JSON.parse(readFileSync(output+'state.json', 'utf8'));
const base = process.env.REVIEW_URL ?? 'http://127.0.0.1:5176';
const name = 'staff_verify_'+Date.now(), pendingName = name+'_pending';
const password = 'Staff-'+randomBytes(12).toString('hex');
const results = [], errors = [], created = [];
const deleted = new Set();
const pass = name => { results.push({name, result:'PASS'}); console.log('PASS '+name); };
const browser = await chromium.launch({headless:true, args:['--no-sandbox']});
const ownerContext = await browser.newContext({viewport:{width:1440,height:1000}, permissions:['clipboard-read','clipboard-write']});
const reviewerContext = await browser.newContext({viewport:{width:1440,height:1000}});
const guestContext = await browser.newContext({viewport:{width:390,height:844}});
const owner = await ownerContext.newPage(), reviewer = await reviewerContext.newPage(), guest = await guestContext.newPage();
for(const page of [owner, reviewer, guest]) { page.setDefaultTimeout(15000); page.on('pageerror', e => errors.push(e.message)); }
let ownerToken = '';
async function api(path, body, token, expected=200) {
  for(let attempt=0;attempt<3;attempt++) {
    const response = await ownerContext.request.post(base+'/api/'+path, {data:body, headers:{Authorization:'Bearer '+token}});
    if(response.status()===429 && attempt<2) {
      console.log('Waiting for rate limit: '+path);
      await new Promise(resolve=>setTimeout(resolve,Math.min(60,Number(response.headers()['retry-after'])||60)*1000));
      continue;
    }
    const data = await response.json(); assert.equal(response.status(), expected, path+': '+JSON.stringify(data)); return data;
  }
}
async function login(page, account, pass) {
  await page.goto(base+'/login');
  await page.getByLabel('账号', {exact:true}).fill(account);
  await page.getByLabel('密码', {exact:true}).fill(pass);
  for(let attempt=0;attempt<3;attempt++) {
    const response = page.waitForResponse(r=>r.url().endsWith('/api/admin/login') && r.request().method()==='POST');
    await page.getByRole('button', {name:'登录审核中心',exact:true}).click();
    const result = await response;
    if(result.status()===429 && attempt<2) {
      console.log('Waiting for login rate limit');
      await new Promise(resolve=>setTimeout(resolve,Math.min(60,Number(result.headers()['retry-after'])||60)*1000));
      continue;
    }
    assert.equal(result.status(),200,await result.text()); break;
  }
  await page.waitForURL('**/reviews');
}
const row = account => owner.locator('[data-member="'+account+'"]');
const dialog = () => owner.locator('dialog[open]');
async function confirmAction(account, action) {
  await row(account).getByRole('button',{name:action,exact:true}).click();
  const response = owner.waitForResponse(r => r.request().method() === 'POST' && /\/api\/admin\/members\/(link|update)$/.test(r.url()));
  await dialog().getByRole('button',{name:'确认',exact:true}).click();
  assert.equal((await response).status(),200);
}
async function invite(account, username) {
  await owner.getByRole('button',{name:'添加成员',exact:true}).click();
  await dialog().getByLabel('姓名',{exact:true}).fill(username);
  await dialog().getByLabel('登录账号',{exact:true}).fill(account);
  await dialog().getByRole('button',{name:'生成邀请',exact:true}).click();
  const input = dialog().getByLabel('激活或重置链接');
  await input.waitFor(); const link = await input.inputValue();
  assert(link.startsWith(base+'/activate#'));
  await dialog().getByRole('button',{name:'复制链接',exact:true}).click();
  assert.equal(await owner.evaluate(() => navigator.clipboard.readText()), link);
  await dialog().getByRole('button',{name:'完成',exact:true}).click();
  await row(account).waitFor();
  const data = await api('admin/members',{query:account},ownerToken);
  created.push(data.items.find(x=>x.account_name===account).id);
  return link;
}
async function activate(page, link, pass) {
  await page.goto(link);
  await page.getByLabel('新密码',{exact:true}).fill(pass);
  await page.getByLabel('确认密码',{exact:true}).fill(pass);
  await page.getByRole('button',{name:'确认设置密码',exact:true}).click();
  await page.getByText('密码已设置',{exact:true}).waitFor();
  assert(!page.url().includes('#'));
}
async function newSubmission() {
  const upload = async (path, filename, mimeType) => {
    const response = await ownerContext.request.post(base+'/api/video/'+path, {
      headers:{Authorization:'Bearer '+state.author},
      multipart:{file:{name:filename,mimeType,buffer:readFileSync(output+filename)}}
    });
    assert.equal(response.status(),200); return response.json();
  };
  const cover = await upload('uploadCover','cover.jpg','image/jpeg');
  const raw = await upload('uploadVideo','fixture.mp4','video/mp4');
  const video = await api('video/publish',{title:'成员管理验证：新审核员审批',play_object_key:raw.object_key,cover_object_key:cover.object_key},state.author);
  for(let i=0;i<60;i++) {
    const data = await api('video/processingStatus',{id:video.id},state.author);
    if(data.status==='pending_review') return video.id;
    assert.notEqual(data.status,'failed'); assert.notEqual(data.status,'published');
    await new Promise(resolve=>setTimeout(resolve,1000));
  }
  throw new Error('test submission processing timed out');
}
try {
  await login(owner,staff.account_name,staff.password);
  ownerToken = await owner.evaluate(()=>sessionStorage.getItem('videohub_review_session'));
  await owner.getByRole('link',{name:'成员管理',exact:true}).click();
  await owner.getByRole('heading',{name:'成员管理',exact:true}).waitFor();
  const link = await invite(name,'浏览器验证审核员');
  await owner.screenshot({path:output+'staff-desktop.png',fullPage:true});
  assert(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  pass('网页添加独立审核员、生成并复制一次性邀请链接');

  await guest.goto(link);
  await guest.getByLabel('新密码',{exact:true}).fill(password);
  await guest.getByLabel('确认密码',{exact:true}).fill(password+'wrong');
  await guest.getByRole('button',{name:'确认设置密码',exact:true}).click();
  await guest.getByText('两次密码不一致',{exact:true}).waitFor();
  await guest.screenshot({path:output+'staff-activation-mobile.png',fullPage:true});
  assert(await guest.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await guest.getByLabel('确认密码',{exact:true}).fill(password);
  await guest.getByRole('button',{name:'确认设置密码',exact:true}).click();
  await guest.getByText('密码已设置',{exact:true}).waitFor();
  await guest.goto(link); await guest.getByRole('alert').waitFor();
  assert((await guest.getByRole('alert').innerText()).includes('已使用'));
  pass('手机激活页、密码确认校验、成功激活及链接不可重复使用');

  await login(reviewer,name,password);
  assert.equal(await reviewer.getByRole('link',{name:'成员管理',exact:true}).count(),0);
  let reviewerToken = await reviewer.evaluate(()=>sessionStorage.getItem('videohub_review_session'));
  await api('admin/members',{},reviewerToken,403);
  await api('account/login',{account_name:name,password},'',401);
  await reviewer.goto(base+'/members'); await reviewer.getByRole('heading',{name:'无访问权限'}).waitFor();
  assert(await reviewer.evaluate(()=>sessionStorage.getItem('videohub_review_session')));
  await reviewer.getByRole('link',{name:'返回视频审核',exact:true}).click();
  pass('审核员不能管理成员，后台账号不能登录社区，拒绝访问不误退出');

  const videoID = await newSubmission();
  await reviewer.goto(base+'/reviews/'+videoID);
  await reviewer.waitForFunction(()=>document.querySelector('video')?.readyState>=2);
  const preview = await reviewer.locator('video').getAttribute('src');
  await reviewer.getByRole('button',{name:'通过并发布',exact:true}).click();
  await reviewer.getByRole('button',{name:'确认发布',exact:true}).click();
  await reviewer.locator('.history').getByText('审核通过',{exact:true}).waitFor();
  const reviewed = await api('admin/detail',{id:videoID},ownerToken);
  assert.equal(reviewed.reviews[0].reviewer_name,'浏览器验证审核员');
  const submission = await api('video/submission',{id:videoID},state.author);
  assert.equal(submission.status,'published');
  pass('新邀请审核员实际预览、通过视频、审核人记录与作者发布状态');

  await owner.getByRole('button',{name:'刷新',exact:true}).click();
  await row(name).getByText('正常',{exact:true}).waitFor();
  await confirmAction(name,'停用成员');
  await row(name).getByText('已停用',{exact:true}).waitFor();
  await api('admin/me',{},reviewerToken,401);
  const mediaResponse = await reviewerContext.request.get(new URL(preview,base).href);
  assert.equal(mediaResponse.status(),403);
  await reviewer.reload(); await reviewer.waitForURL('**/login');
  await confirmAction(name,'启用成员');
  await row(name).getByText('正常',{exact:true}).waitFor();
  await api('admin/me',{},reviewerToken,401);
  pass('停用立即撤销登录与私有预览，重新启用不恢复旧会话');

  await confirmAction(name,'重置密码');
  await dialog().getByLabel('激活或重置链接').waitFor();
  const resetLink = await dialog().getByLabel('激活或重置链接').inputValue();
  await dialog().getByRole('button',{name:'完成',exact:true}).click();
  await activate(guest,resetLink,password+'reset');
  await api('admin/login',{account_name:name,password},'',401);
  await login(reviewer,name,password+'reset');
  await reviewer.getByRole('link',{name:'个人设置',exact:true}).click();
  await reviewer.getByLabel('原密码',{exact:true}).fill(password+'reset');
  await reviewer.getByLabel('新密码',{exact:true}).fill(password+'final');
  await reviewer.getByLabel('确认密码',{exact:true}).fill(password+'final');
  await reviewer.getByRole('button',{name:'修改并重新登录',exact:true}).click();
  await reviewer.waitForURL('**/login');
  pass('网页重置密码、旧密码失效、个人修改密码后重新登录');

  await row(name).getByRole('button',{name:'修改角色',exact:true}).click();
  await dialog().getByLabel('角色',{exact:true}).selectOption('owner');
  await dialog().getByRole('button',{name:'确认',exact:true}).click();
  await row(name).getByText('超级管理员',{exact:true}).waitFor();
  await login(reviewer,name,password+'final');
  await reviewer.getByRole('link',{name:'成员管理',exact:true}).waitFor();
  await row(name).getByRole('button',{name:'修改角色',exact:true}).click();
  await dialog().getByLabel('角色',{exact:true}).selectOption('reviewer');
  await dialog().getByRole('button',{name:'确认',exact:true}).click();
  await row(name).getByText('审核员',{exact:true}).waitFor();
  await reviewer.reload(); await reviewer.waitForURL('**/login');
  pass('网页修改角色、重新登录获得新权限、降权撤销旧登录');

  const revoked = await invite(pendingName,'浏览器验证待激活');
  await confirmAction(pendingName,'撤销链接');
  await guest.goto(revoked); await guest.getByRole('alert').waitFor();
  await confirmAction(pendingName,'重新邀请');
  await dialog().getByLabel('激活或重置链接').waitFor();
  const replacement = await dialog().getByLabel('激活或重置链接').inputValue();
  assert.notEqual(replacement,revoked);
  await dialog().getByRole('button',{name:'完成',exact:true}).click();
  await guest.goto(replacement); await guest.getByRole('button',{name:'确认设置密码'}).waitFor();
  await confirmAction(pendingName,'停用成员');
  await guest.reload(); await guest.getByRole('alert').waitFor();
  pass('撤销邀请、重新邀请、停用待激活账号使链接失效');

  await owner.getByLabel('搜索成员').fill('not_found_'+name);
  await owner.getByRole('button',{name:'搜索',exact:true}).click();
  await owner.getByText('没有符合条件的成员',{exact:true}).waitFor();
  await owner.getByLabel('搜索成员').fill('');
  await owner.getByRole('button',{name:'搜索',exact:true}).click();
  await owner.setViewportSize({width:390,height:844});
  await row(name).waitFor(); await owner.screenshot({path:output+'staff-mobile.png',fullPage:true});
  assert(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await owner.getByRole('link',{name:'操作日志',exact:true}).click();
  await owner.locator('.audit-table tbody tr').first().waitFor();
  await owner.screenshot({path:output+'staff-audit-mobile.png',fullPage:true});
  assert(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  if(await owner.getByRole('button',{name:'下一页',exact:true}).isEnabled()) {
    await owner.getByRole('button',{name:'下一页',exact:true}).click();
    await owner.getByText(/第 2 页/).waitFor();
  }
  await guest.goto(base+'/setup'); await guest.getByText('初始化未开放或已完成',{exact:true}).waitFor();
  assert.deepEqual(errors,[]);
  pass('成员搜索与空状态、日志分页、桌面手机布局、关闭初始化、无浏览器异常');

  await owner.setViewportSize({width:1440,height:1000});
  await owner.getByRole('link',{name:'成员管理',exact:true}).click();
  await row(name).waitFor();
  const me = await api('admin/me',{},ownerToken);
  assert.equal(await row(me.account_name).getByRole('button',{name:'删除成员',exact:true}).count(),0);
  await api('admin/members/delete',{id:me.id,account_name:me.account_name},ownerToken,409);
  const target = (await api('admin/members',{query:name},ownerToken)).items.find(x=>x.account_name===name);
  await login(reviewer,name,password+'final');
  reviewerToken = await reviewer.evaluate(()=>sessionStorage.getItem('videohub_review_session'));
  await api('admin/members/delete',{id:target.id,account_name:name},reviewerToken,403);
  const beforeDelete = await api('admin/detail',{id:videoID},reviewerToken);
  await row(name).getByRole('button',{name:'删除成员',exact:true}).click();
  assert(await dialog().getByRole('button',{name:'确认删除',exact:true}).isDisabled());
  await dialog().getByLabel('确认登录账号').fill('wrong');
  assert(await dialog().getByRole('button',{name:'确认删除',exact:true}).isDisabled());
  await dialog().getByRole('button',{name:'取消',exact:true}).click();
  await row(name).waitFor();
  await row(name).getByRole('button',{name:'删除成员',exact:true}).click();
  await dialog().getByLabel('确认登录账号').fill(name);
  await owner.screenshot({path:output+'staff-delete-desktop.png',fullPage:true});
  await owner.setViewportSize({width:390,height:844});
  await owner.screenshot({path:output+'staff-delete-mobile.png',fullPage:true});
  assert(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await dialog().getByRole('button',{name:'确认删除',exact:true}).click();
  await row(name).waitFor({state:'detached'});
  deleted.add(target.id);
  await api('admin/me',{},reviewerToken,401);
  await api('admin/login',{account_name:name,password:password+'final'},'',401);
  assert.equal((await reviewerContext.request.get(new URL(beforeDelete.video.play_url,base).href)).status(),403);
  await api('admin/members/update',{id:target.id,role:'reviewer',status:'active'},ownerToken,404);
  assert.equal((await api('admin/members',{query:name},ownerToken)).items.some(x=>x.id===target.id),false);
  const afterDelete = await api('admin/detail',{id:videoID},ownerToken);
  assert.deepEqual(afterDelete.reviews,beforeDelete.reviews);
  await reviewer.reload(); await reviewer.waitForURL('**/login');
  pass('删除二次确认与取消、禁止自删和越权、撤销会话与预览、保留真实审核记录');

  await owner.setViewportSize({width:1440,height:1000});
  await confirmAction(pendingName,'启用成员');
  await row(pendingName).getByText('待激活',{exact:true}).waitFor();
  await confirmAction(pendingName,'重新邀请');
  await dialog().getByLabel('激活或重置链接').waitFor();
  const pendingLink = await dialog().getByLabel('激活或重置链接').inputValue();
  await dialog().getByRole('button',{name:'完成',exact:true}).click();
  const pending = (await api('admin/members',{query:pendingName},ownerToken)).items[0];
  await row(pendingName).getByRole('button',{name:'删除成员',exact:true}).click();
  await dialog().getByLabel('确认登录账号').fill(pendingName);
  await dialog().getByRole('button',{name:'确认删除',exact:true}).click();
  await row(pendingName).waitFor({state:'detached'});
  deleted.add(pending.id);
  await guest.goto(pendingLink); await guest.getByRole('alert').waitFor();
  await api('admin/members/invite',{account_name:pendingName,username:'不得复用',role:'reviewer'},ownerToken,409);
  await owner.getByRole('link',{name:'操作日志',exact:true}).click();
  await owner.locator('.audit-table').getByText('删除成员',{exact:true}).first().waitFor();
  const audit = await api('admin/audit',{},ownerToken);
  assert(audit.items.some(x=>x.action==='delete' && x.target_id===pending.id && x.actor_id===me.id));
  assert.deepEqual(errors,[]);
  pass('删除待激活成员后邀请失效、账号禁止复用、删除操作日志可追溯');
} catch(error) {
  await owner.screenshot({path:output+'staff-failure.png',fullPage:true});
  throw error;
} finally {
  for(const id of created) {
    if(deleted.has(id)) continue;
    try { await api('admin/members/update',{id,role:'reviewer',status:'disabled'},ownerToken); } catch(e) { console.error('cleanup:',e.message); }
  }
  writeFileSync(output+'staff-browser-results.json',JSON.stringify({time:new Date().toISOString(),results,errors},null,2));
  await browser.close();
}
