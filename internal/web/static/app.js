/* RedPhone UI logic — vanilla JS + WebSocket. 프레임워크 없음. */
(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const ALL = "__all__"; // 전체 쪽지 대상(가상 피어)
  const state = {
    peers: [],            // [{id,name,ip,httpPort,...}]
    selected: null,       // peer id
    convos: new Map(),    // peerId -> [{dir,text,ts,from}]
    unread: new Map(),    // peerId -> count
  };

  /* ---------------- helpers ---------------- */
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const hhmm = (ts) => {
    const d = ts ? new Date(ts) : new Date();
    return String(d.getHours()).padStart(2, "0") + ":" + String(d.getMinutes()).padStart(2, "0");
  };
  let toastT;
  function toast(msg) {
    const t = $("toast");
    t.textContent = msg;
    t.classList.add("show");
    clearTimeout(toastT);
    toastT = setTimeout(() => t.classList.remove("show"), 2400);
  }
  async function jsonOrNull(res) { try { return await res.json(); } catch { return null; } }

  /* ---------------- peers ---------------- */
  function renderPeers() {
    const ul = $("peers");
    ul.innerHTML = "";
    $("peerCount").textContent = state.peers.length;
    $("peersEmpty").style.display = state.peers.length ? "none" : "block";

    // 맨 위 고정: 전체 쪽지(모든 피어에게 한 번에). 원조 빨간전화기의 상징 기능.
    const all = document.createElement("li");
    all.className = "all" + (state.selected === ALL ? " active" : "");
    const au = state.unread.get(ALL) || 0;
    all.innerHTML =
      `<span class="led on"></span><span class="nm">📢 전체</span>` +
      (au ? `<span class="badge">${au}</span>` : "") +
      `<span class="ip">${state.peers.length}명</span>`;
    all.addEventListener("click", () => selectPeer(ALL));
    ul.appendChild(all);

    for (const p of state.peers) {
      const li = document.createElement("li");
      if (p.id === state.selected) li.className = "active";
      const unread = state.unread.get(p.id) || 0;
      li.innerHTML =
        `<span class="led on"></span><span class="nm">${esc(p.name || p.id)}</span>` +
        (unread ? `<span class="badge">${unread}</span>` : "") +
        `<span class="ip">${esc(p.ip || "")}</span>`;
      li.addEventListener("click", () => selectPeer(p.id));
      ul.appendChild(li);
    }
  }

  function peerById(id) { return state.peers.find((p) => p.id === id); }

  function selectPeer(id) {
    state.selected = id;
    state.unread.set(id, 0);
    $("placeholder").style.display = "none";
    $("composer").hidden = false;
    if (id === ALL) {
      $("chatTitle").textContent = "📢 전체 쪽지";
      $("chatIP").textContent = `${state.peers.length}명에게 전송`;
      $("peerLed").className = "led on";
      $("text").placeholder = "모든 피어에게 보낼 메시지…";
    } else {
      const p = peerById(id);
      $("chatTitle").textContent = p ? (p.name || p.id) : "(오프라인)";
      $("chatIP").textContent = p ? `${p.ip}:${p.httpPort}` : "";
      $("peerLed").className = "led " + (p ? "on" : "off");
      $("text").placeholder = "메시지 입력 후 Enter…";
    }
    renderPeers();
    renderLog();
    closeSide();
    $("text").focus();
  }

  /* ---------------- conversation log ---------------- */
  function pushMsg(peerId, m) {
    if (!state.convos.has(peerId)) state.convos.set(peerId, []);
    state.convos.get(peerId).push(m);
  }
  function renderLog() {
    const log = $("log");
    log.querySelectorAll(".msg,.sys").forEach((n) => n.remove());
    const items = state.convos.get(state.selected) || [];
    for (const m of items) {
      const div = document.createElement("div");
      if (m.dir === "sys") {
        div.className = "sys";
        div.textContent = m.text;
      } else {
        div.className = "msg " + m.dir;
        div.innerHTML = `${esc(m.text)}<span class="meta">${m.dir === "out" ? "나" : esc(m.from || "")} · ${hhmm(m.ts)}</span>`;
      }
      log.appendChild(div);
    }
    log.scrollTop = log.scrollHeight;
  }

  /* ---------------- sending ---------------- */
  async function sendText() {
    const input = $("text");
    const text = input.value.trim();
    if (!text || !state.selected) return;
    input.value = "";
    if (state.selected === ALL) return broadcastText(text);
    try {
      const res = await fetch("/api/send", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ peerId: state.selected, text }),
      });
      if (!res.ok) {
        const e = await jsonOrNull(res);
        toast("전송 실패: " + (e && e.error ? e.error : res.status));
        return;
      }
      pushMsg(state.selected, { dir: "out", text, ts: Date.now() });
      renderLog();
    } catch (err) { toast("전송 오류: " + err.message); }
  }

  async function broadcastText(text) {
    try {
      const res = await fetch("/api/broadcast", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text }),
      });
      const d = await jsonOrNull(res);
      if (!res.ok || !d) { toast("전체 전송 실패"); return; }
      pushMsg(ALL, { dir: "out", text, ts: Date.now() });
      renderLog();
      toast(`전체 전송: ${d.sent}명 성공${d.failed ? `, ${d.failed}명 실패` : ""}`);
    } catch (err) { toast("전체 전송 오류: " + err.message); }
  }

  async function sendFile(file) {
    if (!file) return;
    if (state.selected === ALL) { toast("전체 파일 전송은 미지원 — 피어를 선택하세요"); return; }
    if (!state.selected) return;
    const fd = new FormData();
    fd.append("peerId", state.selected);
    fd.append("file", file);
    toast(`파일 전송 중: ${file.name}`);
    try {
      const res = await fetch("/api/sendfile", { method: "POST", body: fd });
      if (!res.ok) { toast("파일 전송 실패"); return; }
      pushMsg(state.selected, { dir: "sys", text: `📎 파일 전송: ${file.name}` });
      renderLog();
    } catch (err) { toast("파일 오류: " + err.message); }
  }

  /* ---------------- URL share ---------------- */
  async function uploadShare(file) {
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    toast(`공유 링크 생성 중: ${file.name}`);
    try {
      const res = await fetch("/api/share", { method: "POST", body: fd });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) { toast("공유 실패"); return; }
      await loadShares();
      toast("공유 링크 생성됨");
    } catch (err) { toast("공유 오류: " + err.message); }
  }

  async function loadShares() {
    try {
      const res = await fetch("/api/shares");
      const list = (await jsonOrNull(res)) || [];
      const ul = $("shares");
      ul.innerHTML = "";
      for (const s of list) {
        const url = `${location.origin}/s/${s.token}`;
        const li = document.createElement("li");
        li.innerHTML =
          `<div class="top"><span class="k ${esc(s.kind)}">${esc(s.kind)}</span>` +
          `<span class="nm">${esc(s.name)}</span>` +
          `<button class="rm" title="회수">✕</button></div>` +
          `<a href="${esc(url)}" target="_blank" rel="noopener">${esc(url)}</a>` +
          `<button class="copy">링크 복사</button>`;
        li.querySelector(".rm").addEventListener("click", () => revokeShare(s.token));
        li.querySelector(".copy").addEventListener("click", () => copy(url));
        ul.appendChild(li);
      }
    } catch { /* 무시 */ }
  }

  async function revokeShare(token) {
    try {
      await fetch("/api/share/" + token, { method: "DELETE" });
      await loadShares();
      toast("공유 회수됨");
    } catch { toast("회수 실패"); }
  }

  async function copy(text) {
    try { await navigator.clipboard.writeText(text); toast("복사됨"); }
    catch {
      const ta = document.createElement("textarea");
      ta.value = text; document.body.appendChild(ta); ta.select();
      try { document.execCommand("copy"); toast("복사됨"); } catch { toast("복사 실패"); }
      ta.remove();
    }
  }

  /* ---------------- friend IP management ---------------- */
  async function loadTargets() {
    try {
      const res = await fetch("/api/targets");
      const d = await jsonOrNull(res);
      renderTargets((d && d.targets) || []);
    } catch { /* 무시 */ }
  }
  function renderTargets(list) {
    const ul = $("targets");
    ul.innerHTML = "";
    $("targetsEmpty").style.display = list.length ? "none" : "block";
    for (const ip of list) {
      const li = document.createElement("li");
      li.innerHTML =
        `<span class="ip">${esc(ip)}</span>` +
        `<button class="edit" title="변경">✎</button>` +
        `<button class="rm" title="삭제">✕</button>`;
      li.querySelector(".edit").addEventListener("click", () => editTarget(ip));
      li.querySelector(".rm").addEventListener("click", () => removeTarget(ip));
      ul.appendChild(li);
    }
  }
  async function addTarget(ip) {
    ip = (ip || "").trim();
    if (!ip) return;
    try {
      const res = await fetch("/api/targets", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ip }),
      });
      const d = await jsonOrNull(res);
      if (!res.ok) { toast("추가 실패: " + (d && d.error ? d.error : res.status)); return; }
      renderTargets((d && d.targets) || []);
      $("ipInput").value = "";
      toast("친구 IP 추가: " + ip);
    } catch (err) { toast("추가 오류: " + err.message); }
  }
  async function editTarget(oldIp) {
    const next = prompt("새 IP로 변경", oldIp);
    if (next === null) return;
    const nv = next.trim();
    if (!nv || nv === oldIp) return;
    try {
      const res = await fetch("/api/targets", {
        method: "PUT", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ old: oldIp, new: nv }),
      });
      const d = await jsonOrNull(res);
      if (!res.ok) { toast("변경 실패: " + (d && d.error ? d.error : res.status)); return; }
      renderTargets((d && d.targets) || []);
      toast("변경됨: " + nv);
    } catch (err) { toast("변경 오류: " + err.message); }
  }
  async function removeTarget(ip) {
    try {
      const res = await fetch("/api/targets/" + encodeURIComponent(ip), { method: "DELETE" });
      const d = await jsonOrNull(res);
      if (!res.ok) { toast("삭제 실패"); return; }
      renderTargets((d && d.targets) || []);
      toast("삭제됨: " + ip);
    } catch (err) { toast("삭제 오류: " + err.message); }
  }
  async function scanLAN() {
    toast("전체 스캔 중…");
    try {
      const res = await fetch("/api/scan", { method: "POST" });
      const d = await jsonOrNull(res);
      if (!res.ok) { toast("스캔 실패: " + (d && d.error ? d.error : res.status)); return; }
      toast(`스캔: ${d.sent}개 호스트에 HELLO 전송`);
    } catch (err) { toast("스캔 오류: " + err.message); }
  }

  /* ---------------- WebSocket ---------------- */
  function setWS(on) {
    $("wsLed").className = "led " + (on ? "on" : "amber");
    $("wsState").textContent = on ? "온라인" : "재연결 중…";
  }
  function connectWS() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/ws`);
    ws.onopen = () => setWS(true);
    ws.onclose = () => { setWS(false); setTimeout(connectWS, 1500); };
    ws.onerror = () => ws.close();
    ws.onmessage = (ev) => {
      let msg; try { msg = JSON.parse(ev.data); } catch { return; }
      if (msg.type === "peers") {
        state.peers = msg.peers || [];
        renderPeers();
        if (state.selected) {
          const p = peerById(state.selected);
          $("peerLed").className = "led " + (p ? "on" : "off");
          if (!p) $("chatTitle").textContent += " (오프라인)";
        }
      } else if (msg.type === "message" && msg.note) {
        ringPhone();
        const n = msg.note;
        pushMsg(n.fromId, { dir: "in", text: n.text, ts: n.ts, from: n.from });
        if (n.fromId === state.selected) renderLog();
        else { state.unread.set(n.fromId, (state.unread.get(n.fromId) || 0) + 1); renderPeers(); }
        toast(`${n.from}: ${n.text}`.slice(0, 60));
      } else if (msg.type === "file") {
        ringPhone();
        toast(`파일 수신: ${msg.text}`);
        if (state.selected) { pushMsg(state.selected, { dir: "sys", text: `📥 파일 수신: ${msg.text} (downloads/)` }); renderLog(); }
      }
    };
  }
  function ringPhone() {
    const r = $("ring"); r.classList.remove("ring"); void r.offsetWidth; r.classList.add("ring");
  }

  /* ---------------- shutdown ---------------- */
  async function shutdown() {
    if (!confirm("RedPhone을 종료할까요?")) return;
    try { await fetch("/api/shutdown", { method: "POST" }); } catch { /* 종료 중 */ }
    $("bye").hidden = false;
  }

  /* ---------------- sidebar (mobile) ---------------- */
  const openSide = () => { $("side").classList.add("open"); $("scrim").classList.add("show"); };
  const closeSide = () => { $("side").classList.remove("open"); $("scrim").classList.remove("show"); };

  /* ---------------- wiring ---------------- */
  function init() {
    $("sendBtn").addEventListener("click", sendText);
    $("text").addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); sendText(); } });
    $("fileInput").addEventListener("change", (e) => { if (e.target.files[0]) sendFile(e.target.files[0]); e.target.value = ""; });
    $("shareInput").addEventListener("change", (e) => { if (e.target.files[0]) uploadShare(e.target.files[0]); e.target.value = ""; });
    $("exitBtn").addEventListener("click", shutdown);
    $("menuBtn").addEventListener("click", openSide);
    $("scrim").addEventListener("click", closeSide);
    $("ipAddBtn").addEventListener("click", () => addTarget($("ipInput").value));
    $("ipInput").addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); addTarget($("ipInput").value); } });
    $("scanBtn").addEventListener("click", scanLAN);

    // 초기 데이터
    fetch("/api/peers").then(jsonOrNull).then((p) => { state.peers = p || []; renderPeers(); });
    loadShares();
    loadTargets();
    connectWS();
  }
  document.addEventListener("DOMContentLoaded", init);
})();
