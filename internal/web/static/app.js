(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const ALL = "__all__";
  const URL_RE = /(https?:\/\/[^\s<]+)/g;
  const THEME_KEY = "redphone-theme";

  const state = {
    peers: [],
    selected: null,
    convos: new Map(),
    unread: new Map(),
    entryIds: new Set(),
    peerNames: new Map(),
    selfIP: null,
    theme: localStorage.getItem(THEME_KEY) || "dark",
  };

  const modal = {
    resolve: null,
    onKeydown: null,
  };

  const esc = (s) => String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

  const hhmm = (ts) => {
    const d = ts ? new Date(ts) : new Date();
    return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  };

  let toastT;
  function toast(msg) {
    const t = $("toast");
    t.textContent = msg;
    t.classList.add("show");
    clearTimeout(toastT);
    toastT = setTimeout(() => t.classList.remove("show"), 2400);
  }

  async function jsonOrNull(res) {
    try {
      return await res.json();
    } catch {
      return null;
    }
  }

  function peerById(id) {
    return state.peers.find((p) => p.id === id);
  }

  function peerLabel(id) {
    if (id === ALL) return "전체";
    const p = peerById(id);
    return p ? (p.name || p.id) : (state.peerNames.get(id) || id);
  }

  function rememberPeerLabel(id, name) {
    id = (id || "").trim();
    name = (name || "").trim();
    if (!id || !name || id === ALL) return;
    state.peerNames.set(id, name);
  }

  function visiblePeerIds() {
    const ids = [];
    const seen = new Set();

    for (const p of state.peers) {
      if (!p || !p.id || p.id === ALL || seen.has(p.id)) continue;
      ids.push(p.id);
      seen.add(p.id);
    }
    for (const peerID of state.convos.keys()) {
      if (!peerID || peerID === ALL || seen.has(peerID)) continue;
      ids.push(peerID);
      seen.add(peerID);
    }
    return ids;
  }

  function applyTheme(theme) {
    state.theme = theme === "light" ? "light" : "dark";
    document.documentElement.dataset.theme = state.theme;
    localStorage.setItem(THEME_KEY, state.theme);
    $("themeBtn").textContent = state.theme === "light" ? "다크" : "화이트";
  }

  function toggleTheme() {
    applyTheme(state.theme === "light" ? "dark" : "light");
  }

  function setClearButton() {
    const btn = $("clearBtn");
    btn.disabled = !state.selected;
  }

  function upgradeComposerInput() {
    const input = $("text");
    if (!input || input.tagName === "TEXTAREA") return input;

    const textarea = document.createElement("textarea");
    textarea.id = input.id;
    textarea.rows = 1;
    textarea.placeholder = input.placeholder || "메시지를 입력하세요";
    textarea.autocomplete = "off";
    textarea.value = input.value || "";

    input.replaceWith(textarea);
    return textarea;
  }

  function resizeComposerInput() {
    const input = $("text");
    if (!input) return;

    input.style.height = "0px";
    const nextHeight = Math.min(Math.max(input.scrollHeight, 43), 140);
    input.style.height = `${nextHeight}px`;
    input.style.overflowY = input.scrollHeight > 140 ? "auto" : "hidden";
  }

  function renderPeers() {
    const ul = $("peers");
    ul.innerHTML = "";
    $("peerCount").textContent = state.peers.length;
    $("peersEmpty").style.display = visiblePeerIds().length ? "none" : "block";

    const all = document.createElement("li");
    all.className = "all" + (state.selected === ALL ? " active" : "");
    const allUnread = state.unread.get(ALL) || 0;
    all.innerHTML =
      `<span class="led on"></span><span class="nm">전체</span>` +
      (allUnread ? `<span class="badge">${allUnread}</span>` : "") +
      `<span class="ip">${state.peers.length}명</span>`;
    all.addEventListener("click", () => selectPeer(ALL));
    ul.appendChild(all);

    for (const peerID of visiblePeerIds()) {
      const p = peerById(peerID);
      const li = document.createElement("li");
      if (peerID === state.selected) li.className = "active";
      const unread = state.unread.get(peerID) || 0;
      const label = p ? (p.name || p.id) : peerLabel(peerID);
      li.innerHTML =
        `<span class="led ${p ? "on" : "off"}"></span><span class="nm">${esc(label)}</span>` +
        (unread ? `<span class="badge">${unread}</span>` : "") +
        `<span class="ip">${esc(p ? (p.ip || "") : "오프라인")}</span>`;
      li.addEventListener("click", () => selectPeer(peerID));
      ul.appendChild(li);
    }
  }

  function renderTargets(list) {
    const ul = $("targets");
    ul.innerHTML = "";
    $("targetsEmpty").style.display = list.length ? "none" : "block";
    for (const ip of list) {
      const li = document.createElement("li");
      li.innerHTML =
        `<span class="ip">${esc(ip)}</span>` +
        `<button class="edit" title="변경">수정</button>` +
        `<button class="rm" title="삭제">삭제</button>`;
      li.querySelector(".edit").addEventListener("click", () => editTarget(ip));
      li.querySelector(".rm").addEventListener("click", () => removeTarget(ip));
      ul.appendChild(li);
    }
  }

  function pushMsg(peerId, entry) {
    if (!state.convos.has(peerId)) state.convos.set(peerId, []);
    state.convos.get(peerId).push(entry);
  }

  function rebuildEntryIds() {
    state.entryIds = new Set();
    for (const items of state.convos.values()) {
      for (const item of items) {
        if (item && item.id) state.entryIds.add(item.id);
      }
    }
  }

  function removeConversation(peerId) {
    state.convos.delete(peerId);
    state.unread.set(peerId, 0);
    rebuildEntryIds();
    if (state.selected === peerId) renderLog();
    renderPeers();
  }

  function addEntry(entry, opts = {}) {
    const markUnread = opts.markUnread !== false;
    if (!entry || !entry.peerId) return false;
    rememberPeerLabel(entry.peerId, entry.from);
    if (entry.id && state.entryIds.has(entry.id)) return false;
    if (entry.id) state.entryIds.add(entry.id);
    pushMsg(entry.peerId, entry);
    if (entry.peerId === state.selected) {
      renderLog();
    } else if (markUnread && entry.dir === "in") {
      state.unread.set(entry.peerId, (state.unread.get(entry.peerId) || 0) + 1);
      renderPeers();
    }
    return true;
  }

  function removeEntry(entry) {
    if (!entry || !entry.peerId || !entry.id) return false;

    const items = state.convos.get(entry.peerId) || [];
    const next = items.filter((item) => item.id !== entry.id);
    if (next.length === items.length) return false;

    if (next.length) {
      state.convos.set(entry.peerId, next);
    } else {
      state.convos.delete(entry.peerId);
      state.unread.set(entry.peerId, 0);
    }
    if (entry.dir === "in" && state.selected !== entry.peerId) {
      const unread = state.unread.get(entry.peerId) || 0;
      if (unread > 0) state.unread.set(entry.peerId, unread - 1);
    }
    state.entryIds.delete(entry.id);
    renderPeers();
    renderLog();
    return true;
  }

  function appendLinkedText(node, text) {
    let last = 0;
    text.replace(URL_RE, (match, _group, offset) => {
      if (offset > last) node.appendChild(document.createTextNode(text.slice(last, offset)));
      const a = document.createElement("a");
      a.href = match;
      a.target = "_blank";
      a.rel = "noopener noreferrer";
      a.textContent = match;
      node.appendChild(a);
      last = offset + match.length;
      return match;
    });
    if (last < text.length) node.appendChild(document.createTextNode(text.slice(last)));
  }

  function renderLog() {
    const log = $("log");
    log.querySelectorAll(".msg,.sys").forEach((n) => n.remove());
    $("placeholder").style.display = state.selected ? "none" : "block";
    if (!state.selected) return;

    const items = state.convos.get(state.selected) || [];
    for (const m of items) {
      if (m.dir === "sys") {
        const sys = document.createElement("div");
        sys.className = "sys";
        appendLinkedText(sys, m.text || "");
        log.appendChild(sys);
        continue;
      }

      const div = document.createElement("div");
      div.className = `msg ${m.dir}`;

      const actions = document.createElement("div");
      actions.className = "msg-actions";
      const copyBtn = document.createElement("button");
      copyBtn.className = "msg-copy";
      copyBtn.type = "button";
      copyBtn.title = "메시지 복사";
      copyBtn.textContent = "복사";
      copyBtn.addEventListener("click", () => copy(m.text || ""));
      actions.appendChild(copyBtn);

      if (m.id) {
        const deleteBtn = document.createElement("button");
        deleteBtn.className = "msg-delete";
        deleteBtn.type = "button";
        deleteBtn.title = "메시지 삭제";
        deleteBtn.textContent = "삭제";
        deleteBtn.addEventListener("click", () => deleteMessage(m));
        actions.appendChild(deleteBtn);
      }
      div.appendChild(actions);

      const body = document.createElement("div");
      body.className = "msg-text";
      appendLinkedText(body, m.text || "");
      div.appendChild(body);

      const meta = document.createElement("span");
      meta.className = "meta";
      meta.textContent = `${m.dir === "out" ? "나" : (m.from || peerLabel(m.peerId))} · ${hhmm(m.ts)}`;
      div.appendChild(meta);
      log.appendChild(div);
    }
    log.scrollTop = log.scrollHeight;
  }

  function selectPeer(id) {
    state.selected = id;
    state.unread.set(id, 0);
    $("composer").hidden = false;
    $("placeholder").style.display = "none";
    setClearButton();

    if (id === ALL) {
      $("chatTitle").textContent = "전체 대화";
      $("chatIP").textContent = `${state.peers.length}명에게 전송`;
      $("peerLed").className = "led on";
      $("text").placeholder = "모든 피어에게 보낼 메시지";
    } else {
      const p = peerById(id);
      $("chatTitle").textContent = peerLabel(id);
      $("chatIP").textContent = p ? `${p.ip}:${p.httpPort}` : "";
      $("peerLed").className = "led " + (p ? "on" : "off");
      $("text").placeholder = "메시지를 입력하세요";
    }
    renderPeers();
    renderLog();
    closeSide();
    $("text").focus();
  }

  async function sendText() {
    const input = $("text");
    const text = input.value.trim();
    if (!text || !state.selected) return;
    input.value = "";
    resizeComposerInput();
    if (state.selected === ALL) return broadcastText(text);
    try {
      const res = await fetch("/api/send", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ peerId: state.selected, text }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("전송 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      addEntry(data && data.entry);
    } catch (err) {
      toast("전송 오류: " + err.message);
    }
  }

  async function broadcastText(text) {
    try {
      const res = await fetch("/api/broadcast", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) {
        toast("전체 전송 실패");
        return;
      }
      addEntry(data.entry);
      toast(`전체 전송: ${data.sent}명 성공${data.failed ? `, ${data.failed}명 실패` : ""}`);
    } catch (err) {
      toast("전체 전송 오류: " + err.message);
    }
  }

  async function sendFile(file) {
    if (!file) return;
    if (state.selected === ALL) {
      toast("전체 파일 전송은 지원하지 않습니다.");
      return;
    }
    if (!state.selected) return;
    const fd = new FormData();
    fd.append("peerId", state.selected);
    fd.append("file", file);
    toast(`파일 전송 중: ${file.name}`);
    try {
      const res = await fetch("/api/sendfile", { method: "POST", body: fd });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("파일 전송 실패");
        return;
      }
      addEntry(data && data.entry);
    } catch (err) {
      toast("파일 오류: " + err.message);
    }
  }

  async function uploadShare(file) {
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    toast(`공유 링크 생성 중: ${file.name}`);
    try {
      const res = await fetch("/api/share", { method: "POST", body: fd });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) {
        toast("공유 실패");
        return;
      }
      await loadShares();
      toast("공유 링크를 만들었습니다.");
    } catch (err) {
      toast("공유 오류: " + err.message);
    }
  }

  function shareBase() {
    if (state.selfIP) {
      const port = location.port ? `:${location.port}` : "";
      return `${location.protocol}//${state.selfIP}${port}`;
    }
    return location.origin;
  }

  async function loadShares() {
    try {
      const res = await fetch("/api/shares");
      const list = (await jsonOrNull(res)) || [];
      const ul = $("shares");
      ul.innerHTML = "";
      for (const s of list) {
        const url = `${shareBase()}/s/${s.token}`;
        const li = document.createElement("li");
        li.innerHTML =
          `<div class="top"><span class="k ${esc(s.kind)}">${esc(s.kind)}</span>` +
          `<span class="nm">${esc(s.name)}</span>` +
          `<button class="rm" title="회수">회수</button></div>` +
          `<a href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(url)}</a>` +
          `<button class="copy">링크 복사</button>`;
        li.querySelector(".rm").addEventListener("click", () => revokeShare(s.token));
        li.querySelector(".copy").addEventListener("click", () => copy(url));
        ul.appendChild(li);
      }
    } catch {}
  }

  async function revokeShare(token) {
    try {
      await fetch("/api/share/" + token, { method: "DELETE" });
      await loadShares();
      toast("공유를 회수했습니다.");
    } catch {
      toast("회수 실패");
    }
  }

  async function copy(text) {
    try {
      await navigator.clipboard.writeText(text);
      toast("복사했습니다.");
    } catch {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
        toast("복사했습니다.");
      } catch {
        toast("복사 실패");
      }
      ta.remove();
    }
  }

  async function loadSelf() {
    try {
      const res = await fetch("/api/self");
      const data = await jsonOrNull(res);
      if (!data) return;
      if (data.name) $("selfName").textContent = data.name;
      if (data.ip) {
        state.selfIP = data.ip;
        $("myipVal").textContent = data.ip;
        $("myip").hidden = false;
      }
    } catch {}
  }

  async function renameSelf() {
    const current = $("selfName").textContent === "connecting..." ? "" : $("selfName").textContent;
    const result = await showDialog({
      title: "이름 변경",
      message: "피어에게 표시할 이름을 입력하세요.",
      inputValue: current,
      showInput: true,
      confirmText: "저장",
    });
    if (!result.confirmed) return;
    const name = (result.value || "").trim();
    if (!name || name === current) return;

    try {
      const res = await fetch("/api/self/name", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) {
        toast("이름 변경 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      $("selfName").textContent = data.name || name;
      toast("이름을 변경했습니다.");
    } catch (err) {
      toast("이름 변경 오류: " + err.message);
    }
  }

  async function loadHistory() {
    try {
      const res = await fetch("/api/history");
      const list = (await jsonOrNull(res)) || [];
      state.convos = new Map();
      state.unread = new Map();
      state.entryIds = new Set();
      state.peerNames = new Map();
      for (const p of state.peers) rememberPeerLabel(p.id, p.name || p.id);
      for (const entry of list) {
        addEntry(entry, { markUnread: false });
      }
      renderPeers();
      renderLog();
    } catch {}
  }

  async function loadTargets() {
    try {
      const res = await fetch("/api/targets");
      const data = await jsonOrNull(res);
      renderTargets((data && data.targets) || []);
    } catch {}
  }

  async function addTarget(ip) {
    ip = (ip || "").trim();
    if (!ip) return;
    try {
      const res = await fetch("/api/targets", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ip }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("추가 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      renderTargets((data && data.targets) || []);
      $("ipInput").value = "";
      toast("친구 IP를 추가했습니다.");
    } catch (err) {
      toast("추가 오류: " + err.message);
    }
  }

  async function editTarget(oldIp) {
    const result = await showDialog({
      title: "친구 IP 수정",
      message: "새 IP를 입력하세요.",
      inputValue: oldIp,
      showInput: true,
      confirmText: "저장",
    });
    if (!result.confirmed) return;
    const next = result.value.trim();
    if (!next || next === oldIp) return;
    try {
      const res = await fetch("/api/targets", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ old: oldIp, new: next }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("변경 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      renderTargets((data && data.targets) || []);
      toast("친구 IP를 변경했습니다.");
    } catch (err) {
      toast("변경 오류: " + err.message);
    }
  }

  async function removeTarget(ip) {
    const result = await showDialog({
      title: "친구 IP 삭제",
      message: `${ip} 를 목록에서 삭제할까요?`,
      confirmText: "삭제",
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/targets/" + encodeURIComponent(ip), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("삭제 실패");
        return;
      }
      renderTargets((data && data.targets) || []);
      toast("친구 IP를 삭제했습니다.");
    } catch (err) {
      toast("삭제 오류: " + err.message);
    }
  }

  async function scanLAN() {
    toast("전체 스캔 중...");
    try {
      const res = await fetch("/api/scan", { method: "POST" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("스캔 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      toast(`스캔 완료: ${data.sent}곳에 HELLO 전송`);
    } catch (err) {
      toast("스캔 오류: " + err.message);
    }
  }

  function setWS(on) {
    $("wsLed").className = "led " + (on ? "on" : "amber");
    $("wsState").textContent = on ? "온라인" : "재연결 중...";
  }

  function connectWS() {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/ws`);
    ws.onopen = () => setWS(true);
    ws.onclose = () => {
      setWS(false);
      setTimeout(connectWS, 1500);
    };
    ws.onerror = () => ws.close();
    ws.onmessage = (ev) => {
      let msg;
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (msg.type === "peers") {
        state.peers = msg.peers || [];
        for (const p of state.peers) rememberPeerLabel(p.id, p.name || p.id);
        renderPeers();
        if (state.selected && state.selected !== ALL) {
          const p = peerById(state.selected);
          $("peerLed").className = "led " + (p ? "on" : "off");
          $("chatTitle").textContent = peerLabel(state.selected);
          $("chatIP").textContent = p ? `${p.ip}:${p.httpPort}` : "";
        }
      } else if (msg.type === "entry" && msg.entry) {
        const added = addEntry(msg.entry);
        if (added && msg.entry.dir === "in") {
          ringPhone();
          toast(`${msg.entry.from || peerLabel(msg.entry.peerId)}: ${msg.entry.text}`.slice(0, 60));
        }
      } else if (msg.type === "entry_deleted" && msg.entry) {
        removeEntry(msg.entry);
      } else if (msg.type === "history_cleared" && msg.peerId) {
        removeConversation(msg.peerId);
      } else if (msg.type === "file") {
        ringPhone();
        toast(`파일 수신: ${msg.text}`);
      }
    };
  }

  function ringPhone() {
    const r = $("ring");
    r.classList.remove("ring");
    void r.offsetWidth;
    r.classList.add("ring");
  }

  function closeModal(result) {
    $("modalLayer").hidden = true;
    document.removeEventListener("keydown", modal.onKeydown);
    const resolver = modal.resolve;
    modal.resolve = null;
    modal.onKeydown = null;
    if (resolver) resolver(result);
  }

  function showDialog({
    title,
    message,
    confirmText = "확인",
    cancelText = "취소",
    inputValue = "",
    showInput = false,
    danger = false,
  }) {
    $("modalTitle").textContent = title || "확인";
    $("modalMessage").textContent = message || "";
    $("modalConfirm").textContent = confirmText;
    $("modalCancel").textContent = cancelText;
    $("modalCancel").hidden = !cancelText;
    $("modalConfirm").classList.toggle("danger", !!danger);
    $("modalConfirm").classList.toggle("ghost", false);

    const input = $("modalInput");
    input.hidden = !showInput;
    input.value = inputValue || "";

    $("modalLayer").hidden = false;
    if (showInput) {
      setTimeout(() => input.focus(), 0);
      input.select();
    } else {
      setTimeout(() => $("modalConfirm").focus(), 0);
    }

    return new Promise((resolve) => {
      modal.resolve = resolve;
      modal.onKeydown = (e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          closeModal({ confirmed: false, value: input.value });
        } else if (e.key === "Enter" && (showInput || document.activeElement === $("modalConfirm"))) {
          e.preventDefault();
          closeModal({ confirmed: true, value: input.value });
        }
      };
      document.addEventListener("keydown", modal.onKeydown);
    });
  }

  async function clearConversation() {
    if (!state.selected) return;
    const name = state.selected === ALL ? "전체 대화" : peerLabel(state.selected);
    const result = await showDialog({
      title: "대화 삭제",
      message: `${name} 기록을 삭제할까요?`,
      confirmText: "삭제",
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/history/" + encodeURIComponent(state.selected), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("대화 삭제 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      removeConversation(state.selected);
      toast("대화를 삭제했습니다.");
    } catch (err) {
      toast("대화 삭제 오류: " + err.message);
    }
  }

  async function deleteMessage(entry) {
    if (!entry || !entry.id) return;
    const result = await showDialog({
      title: "메시지 삭제",
      message: "이 메시지를 삭제할까요?\n상대방 기록은 삭제되지 않습니다.",
      confirmText: "삭제",
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/history/entry/" + encodeURIComponent(entry.id), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast("메시지 삭제 실패: " + (data && data.error ? data.error : res.status));
        return;
      }
      removeEntry((data && data.entry) || entry);
      toast("메시지를 삭제했습니다.");
    } catch (err) {
      toast("메시지 삭제 오류: " + err.message);
    }
  }

  async function shutdown() {
    const result = await showDialog({
      title: "종료",
      message: "RedPhone을 종료할까요?",
      confirmText: "종료",
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      await fetch("/api/shutdown", { method: "POST" });
    } catch {}
    $("bye").hidden = false;
  }

  const openSide = () => {
    $("side").classList.add("open");
    $("scrim").classList.add("show");
  };

  const closeSide = () => {
    $("side").classList.remove("open");
    $("scrim").classList.remove("show");
  };

  function initModal() {
    $("modalConfirm").addEventListener("click", () => {
      closeModal({ confirmed: true, value: $("modalInput").value });
    });
    $("modalCancel").addEventListener("click", () => {
      closeModal({ confirmed: false, value: $("modalInput").value });
    });
    $("modalLayer").addEventListener("click", (e) => {
      if (e.target === $("modalLayer")) {
        closeModal({ confirmed: false, value: $("modalInput").value });
      }
    });
  }

  function init() {
    applyTheme(state.theme);
    initModal();
    setClearButton();

    const composerInput = upgradeComposerInput();
    resizeComposerInput();

    $("sendBtn").addEventListener("click", sendText);
    composerInput.addEventListener("input", resizeComposerInput);
    composerInput.addEventListener("keydown", (e) => {
      if (e.isComposing || e.keyCode === 229) return;
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendText();
      }
    });
    $("fileInput").addEventListener("change", (e) => {
      if (e.target.files[0]) sendFile(e.target.files[0]);
      e.target.value = "";
    });
    $("shareInput").addEventListener("change", (e) => {
      if (e.target.files[0]) uploadShare(e.target.files[0]);
      e.target.value = "";
    });
    $("exitBtn").addEventListener("click", shutdown);
    $("nameBtn").addEventListener("click", renameSelf);
    $("themeBtn").addEventListener("click", toggleTheme);
    $("clearBtn").addEventListener("click", clearConversation);
    $("menuBtn").addEventListener("click", openSide);
    $("scrim").addEventListener("click", closeSide);
    $("ipAddBtn").addEventListener("click", () => addTarget($("ipInput").value));
    $("ipInput").addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        addTarget($("ipInput").value);
      }
    });
    $("scanBtn").addEventListener("click", scanLAN);
    $("myipCopy").addEventListener("click", () => copy($("myipVal").textContent));

    fetch("/api/peers").then(jsonOrNull).then((p) => {
      state.peers = p || [];
      renderPeers();
    });
    loadHistory();
    loadSelf().then(loadShares);
    loadTargets();
    connectWS();
  }

  document.addEventListener("DOMContentLoaded", init);
})();
