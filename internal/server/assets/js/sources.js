"use strict";
(() => {
  const $ = id => document.getElementById(id);
  const fields = ["url", "name", "item_selector", "title_selector", "link_selector", "date_selector", "description_selector", "pager_selector", "max_pages", "refresh_seconds"];
  let editing = null;
  let sources = [];
  let busy = false;
  let displayedGrant = null;
  const clearCredential=()=>{displayedGrant=null;$("reader-url").value="";$("credential").hidden=true;};
  const messages={AUTH_REQUIRED:"登录已失效，请重新登录。",AUTH_UNAVAILABLE:"身份服务暂时不可用，请稍后重试。",SOURCE_BLOCKED:"此地址不在允许范围内，请使用公网网站。",RULE_NO_MATCH:"没有提取到完整条目，请调整标题和链接的选择器。",SOURCE_INVALID:"无法获取内容，请检查网址、选择器和网站是否可访问。",RATE_LIMITED:"操作较频繁，请稍候再试；也可以先撤销不用的阅读器凭据。",SOURCE_CHANGED:"此来源已被修改，请重新载入后编辑。",SOURCE_NOT_FOUND:"此来源不存在，或你没有修改权限。",SOURCE_LIMIT:"来源容量已达上限，请删除不用的来源或减少配置历史。",SIMPLE_SELECTOR_REQUIRED:"请选择简单的标签、类名或 ID，例如 article.card、.title、a。",SOURCE_COMPLEXITY:"网页较大或结构过深，超出了普通来源的解析范围。",REFRESH_FAILED:"本次刷新失败；如已有成功快照，会继续保留。请稍后重试。"};
  const status = (text, error = false) => { $("status").textContent = text; $("status").classList.toggle("error", error); };
  async function api(path, method = "GET", body) {
    const headers = {"Accept":"application/json"};
    if (method !== "GET") { headers["Content-Type"] = "application/json"; headers["X-RSS-Admin-Action"] = "1"; }
    const response = await fetch(path, {method, headers, redirect:"manual", credentials:"same-origin", body:body === undefined ? undefined : JSON.stringify(body)});
    if(response.type==="opaqueredirect"||response.redirected||(response.status>=300&&response.status<400))throw new Error("登录可能已失效，请刷新页面重新登录。");
    if(response.status===204)return {};
    if(!(response.headers.get("content-type")||"").includes("application/json"))throw new Error("收到非数据响应，请刷新页面重新登录。");
    let data;try{data=await response.json();}catch{throw new Error("响应内容不完整，请稍后重试。");}
    if (!response.ok) throw new Error(messages[data.code] || data.msg || (response.status === 401 ? "请重新登录。" : `请求未完成（${response.status}）`));
    return data;
  }
  function spec() {
    const data = {};
    for (const id of fields) data[id] = $(id).value.trim();
    if (!data.name) data.name = new URL(data.url).hostname;
    data.max_pages = Number(data.max_pages); data.refresh_seconds = Number(data.refresh_seconds);
    return data;
  }
  function button(text, action) { const b = document.createElement("button"); b.type = "button"; b.textContent = text; b.addEventListener("click", () => perform(action)); return b; }
  function row(title, detail) { const r = document.createElement("div"); r.className = "item"; const strong = document.createElement("strong"); strong.textContent = title; r.append(strong); const p = document.createElement("div"); p.className = "meta"; p.textContent = detail; r.append(p); return r; }
  async function load() {
    const [sourceData, grantData] = await Promise.all([api("/api/sources"), api("/api/grants")]);
    if(!Array.isArray(sourceData.sources)||!Array.isArray(grantData.grants))throw new Error("来源列表响应不完整，请稍后重试。");
    sources = sourceData.sources || [];
    $("sources").replaceChildren(); $("grants").replaceChildren();
    if (!sources.length) { const p = document.createElement("p"); p.className="hint"; p.textContent="还没有来源。从左侧添加第一个网站。"; $("sources").append(p); }
    for (const src of sources) {
      const r = row(src.spec ? src.spec.name : src.id, src.public ? "公开来源" : "私人来源");
      r.append(button("生成阅读器链接", () => issue(src.id)));
      if(src.can_manage){const health=document.createElement("div");health.className="meta";health.textContent="快照状态：尚未手动检查";r.append(health);r.append(button("刷新快照",async()=>{const state=await api(`/api/sources/${encodeURIComponent(src.id)}/refresh`,"POST",{});health.textContent=`上次成功：${new Date(state.fetched_at).toLocaleString()} · ${state.stale?"使用之前的快照":"已更新"}`;status("订阅快照已刷新。");}));}
      if (src.spec) {
        r.append(button("编辑", () => { editing=src; for (const id of fields) $(id).value = src.spec[id] ?? ""; $("editor-title").textContent="编辑来源"; $("reset").hidden=false; $("advanced").open=true; $("url").focus(); }));
        if (src.previous && src.previous.length) r.append(button("恢复上一版", async () => { if (!window.confirm("恢复这个来源的上一个配置版本？")) return; await api(`/api/sources/${encodeURIComponent(src.id)}/rollback`, "POST", {revision:src.revision}); await load(); status("已恢复上一个版本。"); }));
        r.append(button("删除来源",async()=>{if(!window.confirm("删除此来源并撤销它的所有阅读器凭据？"))return;await api(`/api/sources/${encodeURIComponent(src.id)}/delete`,"POST",{revision:src.revision});if(displayedGrant&&displayedGrant.source===src.id)clearCredential();if(editing&&editing.id===src.id){editing=null;$("source-form").reset();$("editor-title").textContent="添加来源";$("reset").hidden=true;}await load();status("来源及其阅读器凭据已删除。");}));
      }
      $("sources").append(r);
    }
    const grants = grantData.grants || [];
    if(displayedGrant&&!grants.some(g=>g.id===displayedGrant.id))clearCredential();
    if (!grants.length) { const p=document.createElement("p"); p.className="hint"; p.textContent="尚无有效阅读器凭据。"; $("grants").append(p); }
    for (const g of grants) { const r=row(g.label || "阅读器", `${g.source} · 到期 ${new Date(g.expires_at).toLocaleDateString()}`); r.append(button("撤销", async () => { if (!window.confirm("撤销这个阅读器凭据？")) return; await api(`/api/grants/${encodeURIComponent(g.id)}/revoke`, "POST", {});if(displayedGrant&&displayedGrant.id===g.id)clearCredential();await load(); status("已撤销。其他阅读器凭据继续有效。"); })); $("grants").append(r); }
  }
  function showPreview(items) { const wrap=$("preview-result"); wrap.replaceChildren(); const title=document.createElement("h3"); title.textContent=`预览 ${items.length} 条内容`; const list=document.createElement("ol"); for (const item of items) { const li=document.createElement("li"); const heading=document.createElement("strong"); heading.textContent=item.title || "无标题"; const link=document.createElement("div"); link.className="meta"; link.textContent=item.link || "未提取到链接"; li.append(heading,link); list.append(li); } wrap.append(title,list); }
  async function issue(id) { const label=window.prompt("为这个阅读器起一个名字", "我的阅读器"); if (label===null) return; const result=await api(`/api/sources/${encodeURIComponent(id)}/grants`, "POST", {formats:["rss"],days:90,label}); if(!result.id||!result.url)throw new Error("阅读器凭据响应不完整，请重试。");displayedGrant={id:result.id,source:id};$("reader-url").value=result.url; $("credential").hidden=false; $("reader-url").focus(); $("reader-url").select(); await load(); status("订阅链接已生成，可复制到阅读器。它只显示这一次。"); }
  async function perform(fn) { if(busy)return;busy=true;document.querySelectorAll("button").forEach(b=>b.disabled=true); try { await fn(); } catch (error) { status(error.message,true); } finally { busy=false;document.querySelectorAll("button").forEach(b=>b.disabled=false); } }
  $("template").addEventListener("change", () => { const kind=$("template").value; if (kind!=="custom") { $("item_selector").value=kind==="article"?"article":".post"; $("title_selector").value="h2"; $("link_selector").value="a"; } else $("advanced").open=true; });
  $("preview").addEventListener("click", () => perform(async () => { status("正在提取预览…"); const result=await api("/api/sources/preview","POST",{spec:spec()}); showPreview(result.items); status("请确认标题和链接，再保存来源。"); }));
  $("source-form").addEventListener("submit", event => { event.preventDefault(); perform(async () => { status("正在检查并保存…"); const data={spec:spec()}; if(editing)data.revision=editing.revision; const result=await api(editing?`/api/sources/${encodeURIComponent(editing.id)}`:"/api/sources","POST",data);if(!result.source||!result.source.id||!Number.isSafeInteger(result.source.revision))throw new Error("来源保存响应不完整，请重新载入确认状态。"); editing=result.source; $("editor-title").textContent="编辑来源"; $("reset").hidden=false; await load(); status("来源已保存，仅自己可见。"); await issue(result.source.id); }); });
  $("reset").addEventListener("click", () => { editing=null; $("source-form").reset(); $("reset").hidden=true; $("editor-title").textContent="添加来源"; $("preview-result").replaceChildren(); clearCredential(); });
  $("copy").addEventListener("click", () => perform(async () => { $("reader-url").select(); if(navigator.clipboard){await navigator.clipboard.writeText($("reader-url").value);status("已复制。");}else status("链接已选中，请按复制快捷键。"); }));
  perform(load);
})();
