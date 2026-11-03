"use strict";
(() => {
  const $ = id => document.getElementById(id);
  const fields = ["url", "name", "item_selector", "title_selector", "link_selector", "date_selector", "description_selector", "pager_selector", "max_pages", "refresh_seconds"];
  let editing = null;
  let sources = [];
  const status = (text, error = false) => { $("status").textContent = text; $("status").classList.toggle("error", error); };
  async function api(path, method = "GET", body) {
    const headers = {"Accept":"application/json"};
    if (method !== "GET") { headers["Content-Type"] = "application/json"; headers["X-RSS-Admin-Action"] = "1"; }
    const response = await fetch(path, {method, headers, credentials:"same-origin", body:body === undefined ? undefined : JSON.stringify(body)});
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.msg || (response.status === 401 ? "请重新登录。" : `请求未完成（${response.status}）`));
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
    sources = sourceData.sources || [];
    $("sources").replaceChildren(); $("grants").replaceChildren();
    if (!sources.length) { const p = document.createElement("p"); p.className="hint"; p.textContent="还没有来源。从左侧添加第一个网站。"; $("sources").append(p); }
    for (const src of sources) {
      const r = row(src.spec ? src.spec.name : src.id, src.public ? "公开来源" : "私人来源");
      r.append(button("生成阅读器链接", () => issue(src.id)));
      if (src.spec) {
        r.append(button("编辑", () => { editing=src; for (const id of fields) $(id).value = src.spec[id] ?? ""; $("editor-title").textContent="编辑来源"; $("reset").hidden=false; $("advanced").open=true; $("url").focus(); }));
        if (src.previous && src.previous.length) r.append(button("恢复上一版", async () => { if (!window.confirm("恢复这个来源的上一个配置版本？")) return; await api(`/api/sources/${encodeURIComponent(src.id)}/rollback`, "POST", {revision:src.revision}); await load(); status("已恢复上一个版本。"); }));
      }
      $("sources").append(r);
    }
    const grants = grantData.grants || [];
    if (!grants.length) { const p=document.createElement("p"); p.className="hint"; p.textContent="尚无有效阅读器凭据。"; $("grants").append(p); }
    for (const g of grants) { const r=row(g.label || "阅读器", `${g.source} · 到期 ${new Date(g.expires_at).toLocaleDateString()}`); r.append(button("撤销", async () => { if (!window.confirm("撤销这个阅读器凭据？")) return; await api(`/api/grants/${encodeURIComponent(g.id)}/revoke`, "POST", {}); await load(); status("已撤销。其他阅读器凭据继续有效。"); })); $("grants").append(r); }
  }
  function showPreview(items) { const wrap=$("preview-result"); wrap.replaceChildren(); const title=document.createElement("h3"); title.textContent=`预览 ${items.length} 条内容`; const list=document.createElement("ol"); for (const item of items) { const li=document.createElement("li"); const heading=document.createElement("strong"); heading.textContent=item.title || "无标题"; const link=document.createElement("div"); link.className="meta"; link.textContent=item.link || "未提取到链接"; li.append(heading,link); list.append(li); } wrap.append(title,list); }
  async function issue(id) { const label=window.prompt("为这个阅读器起一个名字", "我的阅读器"); if (label===null) return; const result=await api(`/api/sources/${encodeURIComponent(id)}/grants`, "POST", {formats:["rss"],days:90,label}); $("reader-url").value=result.url; $("credential").hidden=false; $("reader-url").focus(); $("reader-url").select(); await load(); status("订阅链接已生成，可复制到阅读器。它只显示这一次。"); }
  async function perform(fn) { const buttons=[...document.querySelectorAll("button")]; buttons.forEach(b=>b.disabled=true); try { await fn(); } catch (error) { status(error.message,true); } finally { buttons.forEach(b=>b.disabled=false); } }
  $("template").addEventListener("change", () => { const kind=$("template").value; if (kind!=="custom") { $("item_selector").value=kind==="article"?"article":".post"; $("title_selector").value="h2"; $("link_selector").value="a"; } else $("advanced").open=true; });
  $("preview").addEventListener("click", () => perform(async () => { status("正在提取预览…"); const result=await api("/api/sources/preview","POST",{spec:spec()}); showPreview(result.items); status("请确认标题和链接，再保存来源。"); }));
  $("source-form").addEventListener("submit", event => { event.preventDefault(); perform(async () => { status("正在检查并保存…"); const data={spec:spec()}; if(editing)data.revision=editing.revision; const result=await api(editing?`/api/sources/${encodeURIComponent(editing.id)}`:"/api/sources","POST",data); editing=result.source; $("editor-title").textContent="编辑来源"; $("reset").hidden=false; await load(); status("来源已保存，仅自己可见。"); await issue(result.source.id); }); });
  $("reset").addEventListener("click", () => { editing=null; $("source-form").reset(); $("reset").hidden=true; $("editor-title").textContent="添加来源"; $("preview-result").replaceChildren(); $("credential").hidden=true; });
  $("copy").addEventListener("click", () => perform(async () => { $("reader-url").select(); if(navigator.clipboard){await navigator.clipboard.writeText($("reader-url").value);status("已复制。");}else status("链接已选中，请按复制快捷键。"); }));
  perform(load);
})();
