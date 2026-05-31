(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const ALL = "__all__";
  const URL_RE = /(https?:\/\/[^\s<]+)/g;
  const THEME_KEY = "redphone-theme";
  const LANG_KEY = "redphone-lang";
  const M = {
    ko: {
      langButton: "EN",
      langTitle: "언어 변경",
      themeLight: "화이트",
      themeDark: "다크",
      themeTitle: "테마 전환",
      renameButton: "이름",
      renameTitle: "이름 변경",
      exit: "종료",
      exitTitle: "종료",
      wsOnline: "온라인",
      wsReconnecting: "재연결 중...",
      wsConnecting: "연결 중...",
      peers: "피어",
      peersEmpty: "같은 LAN의 RedPhone을 찾는 중입니다.",
      friendIp: "친구 IP",
      scanAll: "전체 스캔",
      scanTitle: "서브넷 전체에 HELLO를 보냅니다",
      myIp: "내 IP",
      copy: "복사",
      ipPlaceholder: "예: 192.168.0.42",
      add: "추가",
      targetsEmpty: "브로드캐스트가 막힌 망이면 친구 IP를 직접 추가하세요.",
      share: "URL 공유",
      shareCreate: "파일 공유 링크 만들기",
      menu: "메뉴",
      selectPeer: "피어를 선택하세요",
      clearConversation: "대화 삭제",
      placeholder: "왼쪽에서 피어를 고르면 대화를 볼 수 있습니다.<br />브라우저를 다시 열어도 히스토리는 유지됩니다.",
      attach: "파일",
      attachTitle: "파일 전송",
      inputPlaceholder: "메시지를 입력하세요",
      send: "전송",
      confirm: "확인",
      cancel: "취소",
      bye: "RedPhone이 종료되었습니다.<br />창을 닫아도 됩니다.",
      all: "전체",
      people: "명",
      offline: "오프라인",
      edit: "수정",
      editTitle: "변경",
      delete: "삭제",
      me: "나",
      allConversation: "전체 대화",
      sendToAll: (n) => `${n}명에게 전송`,
      sendToAllPlaceholder: "모든 피어에게 보낼 메시지",
      toastSendFailed: (x) => `전송 실패: ${x}`,
      toastSendError: (x) => `전송 오류: ${x}`,
      toastBroadcastFailed: "전체 전송 실패",
      toastBroadcastResult: (s, f) => `전체 전송: ${s}명 성공${f ? `, ${f}명 실패` : ""}`,
      toastBroadcastError: (x) => `전체 전송 오류: ${x}`,
      toastBroadcastFileUnsupported: "전체 파일 전송은 지원하지 않습니다.",
      toastFileSending: (x) => `파일 전송 중: ${x}`,
      toastFileSendFailed: "파일 전송 실패",
      toastFileError: (x) => `파일 오류: ${x}`,
      toastShareCreating: (x) => `공유 링크 생성 중: ${x}`,
      toastShareFailed: "공유 실패",
      toastShareCreated: "공유 링크를 만들었습니다.",
      toastShareError: (x) => `공유 오류: ${x}`,
      revoke: "회수",
      copyLink: "링크 복사",
      toastShareRevoked: "공유를 회수했습니다.",
      toastRevokeFailed: "회수 실패",
      toastCopied: "복사했습니다.",
      toastCopyFailed: "복사 실패",
      renamePromptTitle: "이름 변경",
      renamePromptMessage: "피어에게 표시할 이름을 입력하세요.",
      save: "저장",
      toastRenameFailed: (x) => `이름 변경 실패: ${x}`,
      toastRenamed: "이름을 변경했습니다.",
      toastRenameError: (x) => `이름 변경 오류: ${x}`,
      toastAddFailed: (x) => `추가 실패: ${x}`,
      toastAdded: "친구 IP를 추가했습니다.",
      toastAddError: (x) => `추가 오류: ${x}`,
      editTargetTitle: "친구 IP 수정",
      editTargetMessage: "새 IP를 입력하세요.",
      toastEditFailed: (x) => `변경 실패: ${x}`,
      toastEdited: "친구 IP를 변경했습니다.",
      toastEditError: (x) => `변경 오류: ${x}`,
      removeTargetTitle: "친구 IP 삭제",
      removeTargetMessage: (ip) => `${ip} 를 목록에서 삭제할까요?`,
      toastDeleteFailed: "삭제 실패",
      toastDeleted: "친구 IP를 삭제했습니다.",
      toastDeleteError: (x) => `삭제 오류: ${x}`,
      toastScanning: "전체 스캔 중...",
      toastScanFailed: (x) => `스캔 실패: ${x}`,
      toastScanDone: (n) => `스캔 완료: ${n}곳에 HELLO 전송`,
      toastScanError: (x) => `스캔 오류: ${x}`,
      toastFileReceived: (x) => `파일 수신: ${x}`,
      clearTitle: "대화 삭제",
      clearMessage: (x) => `${x} 기록을 삭제할까요?`,
      toastClearFailed: (x) => `대화 삭제 실패: ${x}`,
      toastCleared: "대화를 삭제했습니다.",
      toastClearError: (x) => `대화 삭제 오류: ${x}`,
      deleteMessageTitle: "메시지 삭제",
      deleteMessageBody: "이 메시지를 삭제할까요?\n상대방 기록은 삭제되지 않습니다.",
      toastMessageDeleteFailed: (x) => `메시지 삭제 실패: ${x}`,
      toastMessageDeleted: "메시지를 삭제했습니다.",
      toastMessageDeleteError: (x) => `메시지 삭제 오류: ${x}`,
      shutdownTitle: "종료",
      shutdownBody: "RedPhone을 종료할까요?",
      helpTitle: "도움말",
      helpBody: "Enter: 메시지 전송\nShift+Enter: 줄바꿈\nF1: 이 도움말 열기\n\n이름 버튼: 내 표시 이름 변경\n복사: 메시지 텍스트 복사\n삭제: 내 로컬 기록에서만 삭제",
      close: "닫기",
      nameRequired: "이름을 입력하세요.",
    },
    en: {
      langButton: "KO",
      langTitle: "Change language",
      themeLight: "Light",
      themeDark: "Dark",
      themeTitle: "Change theme",
      renameButton: "Name",
      renameTitle: "Change name",
      exit: "Exit",
      exitTitle: "Exit",
      wsOnline: "Online",
      wsReconnecting: "Reconnecting...",
      wsConnecting: "Connecting...",
      peers: "Peers",
      peersEmpty: "Looking for RedPhone peers on this LAN.",
      friendIp: "Friend IP",
      scanAll: "Scan All",
      scanTitle: "Send HELLO to the whole subnet",
      myIp: "My IP",
      copy: "Copy",
      ipPlaceholder: "e.g. 192.168.0.42",
      add: "Add",
      targetsEmpty: "If broadcast is blocked on this network, add a friend's IP manually.",
      share: "URL Share",
      shareCreate: "Create file share link",
      menu: "Menu",
      selectPeer: "Select a peer",
      clearConversation: "Delete Chat",
      placeholder: "Choose a peer on the left to view the conversation.<br />History stays available when you reopen the browser.",
      attach: "File",
      attachTitle: "Send file",
      inputPlaceholder: "Type a message",
      send: "Send",
      confirm: "OK",
      cancel: "Cancel",
      bye: "RedPhone has stopped.<br />You can close this window.",
      all: "All",
      people: "",
      offline: "Offline",
      edit: "Edit",
      editTitle: "Edit",
      delete: "Delete",
      me: "Me",
      allConversation: "All Chat",
      sendToAll: (n) => `Send to ${n} peer${n === 1 ? "" : "s"}`,
      sendToAllPlaceholder: "Message to all peers",
      toastSendFailed: (x) => `Send failed: ${x}`,
      toastSendError: (x) => `Send error: ${x}`,
      toastBroadcastFailed: "Broadcast failed",
      toastBroadcastResult: (s, f) => `Broadcast: ${s} sent${f ? `, ${f} failed` : ""}`,
      toastBroadcastError: (x) => `Broadcast error: ${x}`,
      toastBroadcastFileUnsupported: "Broadcast file transfer is not supported.",
      toastFileSending: (x) => `Sending file: ${x}`,
      toastFileSendFailed: "File transfer failed",
      toastFileError: (x) => `File error: ${x}`,
      toastShareCreating: (x) => `Creating share link: ${x}`,
      toastShareFailed: "Share failed",
      toastShareCreated: "Share link created.",
      toastShareError: (x) => `Share error: ${x}`,
      revoke: "Revoke",
      copyLink: "Copy Link",
      toastShareRevoked: "Share revoked.",
      toastRevokeFailed: "Revoke failed",
      toastCopied: "Copied.",
      toastCopyFailed: "Copy failed",
      renamePromptTitle: "Change Name",
      renamePromptMessage: "Enter the name shown to peers.",
      save: "Save",
      toastRenameFailed: (x) => `Rename failed: ${x}`,
      toastRenamed: "Name updated.",
      toastRenameError: (x) => `Rename error: ${x}`,
      toastAddFailed: (x) => `Add failed: ${x}`,
      toastAdded: "Friend IP added.",
      toastAddError: (x) => `Add error: ${x}`,
      editTargetTitle: "Edit Friend IP",
      editTargetMessage: "Enter the new IP.",
      toastEditFailed: (x) => `Edit failed: ${x}`,
      toastEdited: "Friend IP updated.",
      toastEditError: (x) => `Edit error: ${x}`,
      removeTargetTitle: "Delete Friend IP",
      removeTargetMessage: (ip) => `Delete ${ip} from the list?`,
      toastDeleteFailed: "Delete failed",
      toastDeleted: "Friend IP deleted.",
      toastDeleteError: (x) => `Delete error: ${x}`,
      toastScanning: "Scanning subnet...",
      toastScanFailed: (x) => `Scan failed: ${x}`,
      toastScanDone: (n) => `Scan complete: HELLO sent to ${n} hosts`,
      toastScanError: (x) => `Scan error: ${x}`,
      toastFileReceived: (x) => `File received: ${x}`,
      clearTitle: "Delete Chat",
      clearMessage: (x) => `Delete the chat history for ${x}?`,
      toastClearFailed: (x) => `Delete chat failed: ${x}`,
      toastCleared: "Chat deleted.",
      toastClearError: (x) => `Delete chat error: ${x}`,
      deleteMessageTitle: "Delete Message",
      deleteMessageBody: "Delete this message?\nThe peer's history is not deleted.",
      toastMessageDeleteFailed: (x) => `Delete message failed: ${x}`,
      toastMessageDeleted: "Message deleted.",
      toastMessageDeleteError: (x) => `Delete message error: ${x}`,
      shutdownTitle: "Exit",
      shutdownBody: "Exit RedPhone?",
      helpTitle: "Help",
      helpBody: "Enter: Send message\nShift+Enter: New line\nF1: Open this help\n\nName button: Change your display name\nCopy: Copy message text\nDelete: Delete from local history only",
      close: "Close",
      nameRequired: "Enter a name.",
    },
  };

  function defaultLanguage() {
    const saved = localStorage.getItem(LANG_KEY);
    if (saved === "ko" || saved === "en") return saved;
    return (navigator.language || "").toLowerCase().startsWith("ko") ? "ko" : "en";
  }

  const state = {
    peers: [],
    selected: null,
    convos: new Map(),
    unread: new Map(),
    entryIds: new Set(),
    peerNames: new Map(),
    targets: [],
    selfIP: null,
    theme: localStorage.getItem(THEME_KEY) || "dark",
    lang: defaultLanguage(),
  };

  const modal = {
    resolve: null,
    onKeydown: null,
  };

  const esc = (s) => String(s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const t = (key, ...args) => {
    const value = M[state.lang][key];
    return typeof value === "function" ? value(...args) : value;
  };
  const setHTML = (id, value) => {
    const el = $(id);
    if (el) el.innerHTML = value;
  };

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
    if (id === ALL) return t("all");
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
    $("themeBtn").textContent = state.theme === "light" ? t("themeDark") : t("themeLight");
  }

  function toggleTheme() {
    applyTheme(state.theme === "light" ? "dark" : "light");
  }

  function applyLanguage(lang) {
    state.lang = lang === "ko" ? "ko" : "en";
    localStorage.setItem(LANG_KEY, state.lang);
    document.documentElement.lang = state.lang;

    $("nameBtn").textContent = t("renameButton");
    $("nameBtn").title = t("renameTitle");
    $("langBtn").textContent = t("langButton");
    $("langBtn").title = t("langTitle");
    $("themeBtn").title = t("themeTitle");
    $("exitBtn").textContent = t("exit");
    $("exitBtn").title = t("exitTitle");
    $("wsState").textContent = t("wsConnecting");
    $("peersLabel").textContent = t("peers");
    $("peersEmpty").textContent = t("peersEmpty");
    $("targetsLabel").textContent = t("friendIp");
    $("scanBtn").textContent = t("scanAll");
    $("scanBtn").title = t("scanTitle");
    $("myipLabel").textContent = t("myIp");
    $("myipCopy").textContent = t("copy");
    $("myipCopy").title = t("copy");
    $("ipInput").placeholder = t("ipPlaceholder");
    $("ipAddBtn").textContent = t("add");
    $("targetsEmpty").textContent = t("targetsEmpty");
    $("shareLabel").textContent = t("share");
    $("shareCreateLabel").textContent = t("shareCreate");
    $("menuBtn").textContent = t("menu");
    $("clearBtn").textContent = t("clearConversation");
    setHTML("placeholderText", t("placeholder"));
    $("attachLabel").title = t("attachTitle");
    $("attachText").textContent = t("attach");
    $("sendBtn").textContent = t("send");
    $("modalCancel").textContent = t("cancel");
    $("modalConfirm").textContent = t("confirm");
    setHTML("byeText", t("bye"));
    $("text").placeholder = state.selected === ALL ? t("sendToAllPlaceholder") : t("inputPlaceholder");

    applyTheme(state.theme);
    setWS($("wsLed").classList.contains("on"));
    renderPeers();
    renderTargets(state.targets || []);
    renderLog();
    if (!state.selected) $("chatTitle").textContent = t("selectPeer");
  }

  function toggleLanguage() {
    applyLanguage(state.lang === "ko" ? "en" : "ko");
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
    textarea.placeholder = input.placeholder || t("inputPlaceholder");
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
      `<span class="led on"></span><span class="nm">${t("all")}</span>` +
      (allUnread ? `<span class="badge">${allUnread}</span>` : "") +
      `<span class="ip">${t("sendToAll", state.peers.length)}</span>`;
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
        `<span class="ip">${esc(p ? (p.ip || "") : t("offline"))}</span>`;
      li.addEventListener("click", () => selectPeer(peerID));
      ul.appendChild(li);
    }
  }

  function renderTargets(list) {
    state.targets = list.slice();
    const ul = $("targets");
    ul.innerHTML = "";
    $("targetsEmpty").style.display = list.length ? "none" : "block";
    for (const ip of list) {
      const li = document.createElement("li");
      li.innerHTML =
        `<span class="ip">${esc(ip)}</span>` +
        `<button class="edit" title="${t("editTitle")}">${t("edit")}</button>` +
        `<button class="rm" title="${t("delete")}">${t("delete")}</button>`;
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
      copyBtn.title = t("copy");
      copyBtn.textContent = t("copy");
      copyBtn.addEventListener("click", () => copy(m.text || ""));
      actions.appendChild(copyBtn);

      if (m.id) {
        const deleteBtn = document.createElement("button");
        deleteBtn.className = "msg-delete";
        deleteBtn.type = "button";
        deleteBtn.title = t("delete");
        deleteBtn.textContent = t("delete");
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
      meta.textContent = `${m.dir === "out" ? t("me") : (m.from || peerLabel(m.peerId))} · ${hhmm(m.ts)}`;
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
      $("chatTitle").textContent = t("allConversation");
      $("chatIP").textContent = t("sendToAll", state.peers.length);
      $("peerLed").className = "led on";
      $("text").placeholder = t("sendToAllPlaceholder");
    } else {
      const p = peerById(id);
      $("chatTitle").textContent = peerLabel(id);
      $("chatIP").textContent = p ? `${p.ip}:${p.httpPort}` : "";
      $("peerLed").className = "led " + (p ? "on" : "off");
      $("text").placeholder = t("inputPlaceholder");
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
        toast(t("toastSendFailed", data && data.error ? data.error : res.status));
        return;
      }
      addEntry(data && data.entry);
    } catch (err) {
      toast(t("toastSendError", err.message));
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
        toast(t("toastBroadcastFailed"));
        return;
      }
      addEntry(data.entry);
      toast(t("toastBroadcastResult", data.sent, data.failed));
    } catch (err) {
      toast(t("toastBroadcastError", err.message));
    }
  }

  async function sendFile(file) {
    if (!file) return;
    if (state.selected === ALL) {
      toast(t("toastBroadcastFileUnsupported"));
      return;
    }
    if (!state.selected) return;
    const fd = new FormData();
    fd.append("peerId", state.selected);
    fd.append("file", file);
    toast(t("toastFileSending", file.name));
    try {
      const res = await fetch("/api/sendfile", { method: "POST", body: fd });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast(t("toastFileSendFailed"));
        return;
      }
      addEntry(data && data.entry);
    } catch (err) {
      toast(t("toastFileError", err.message));
    }
  }

  async function uploadShare(file) {
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    toast(t("toastShareCreating", file.name));
    try {
      const res = await fetch("/api/share", { method: "POST", body: fd });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) {
        toast(t("toastShareFailed"));
        return;
      }
      await loadShares();
      toast(t("toastShareCreated"));
    } catch (err) {
      toast(t("toastShareError", err.message));
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
          `<button class="rm" title="${t("revoke")}">${t("revoke")}</button></div>` +
          `<a href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(url)}</a>` +
          `<button class="copy">${t("copyLink")}</button>`;
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
      toast(t("toastShareRevoked"));
    } catch {
      toast(t("toastRevokeFailed"));
    }
  }

  async function copy(text) {
    try {
      await navigator.clipboard.writeText(text);
      toast(t("toastCopied"));
    } catch {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
        toast(t("toastCopied"));
      } catch {
        toast(t("toastCopyFailed"));
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
      title: t("renamePromptTitle"),
      message: t("renamePromptMessage"),
      inputValue: current,
      showInput: true,
      confirmText: t("save"),
    });
    if (!result.confirmed) return;
    const name = (result.value || "").trim();
    if (!name) {
      toast(t("nameRequired"));
      return;
    }
    if (name === current) return;

    try {
      const res = await fetch("/api/self/name", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const data = await jsonOrNull(res);
      if (!res.ok || !data) {
        toast(t("toastRenameFailed", data && data.error ? data.error : res.status));
        return;
      }
      $("selfName").textContent = data.name || name;
      toast(t("toastRenamed"));
    } catch (err) {
      toast(t("toastRenameError", err.message));
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
        toast(t("toastAddFailed", data && data.error ? data.error : res.status));
        return;
      }
      renderTargets((data && data.targets) || []);
      $("ipInput").value = "";
      toast(t("toastAdded"));
    } catch (err) {
      toast(t("toastAddError", err.message));
    }
  }

  async function editTarget(oldIp) {
    const result = await showDialog({
      title: t("editTargetTitle"),
      message: t("editTargetMessage"),
      inputValue: oldIp,
      showInput: true,
      confirmText: t("save"),
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
        toast(t("toastEditFailed", data && data.error ? data.error : res.status));
        return;
      }
      renderTargets((data && data.targets) || []);
      toast(t("toastEdited"));
    } catch (err) {
      toast(t("toastEditError", err.message));
    }
  }

  async function removeTarget(ip) {
    const result = await showDialog({
      title: t("removeTargetTitle"),
      message: t("removeTargetMessage", ip),
      confirmText: t("delete"),
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/targets/" + encodeURIComponent(ip), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast(t("toastDeleteFailed"));
        return;
      }
      renderTargets((data && data.targets) || []);
      toast(t("toastDeleted"));
    } catch (err) {
      toast(t("toastDeleteError", err.message));
    }
  }

  async function scanLAN() {
    toast(t("toastScanning"));
    try {
      const res = await fetch("/api/scan", { method: "POST" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast(t("toastScanFailed", data && data.error ? data.error : res.status));
        return;
      }
      toast(t("toastScanDone", data.sent));
    } catch (err) {
      toast(t("toastScanError", err.message));
    }
  }

  function setWS(on) {
    $("wsLed").className = "led " + (on ? "on" : "amber");
    $("wsState").textContent = on ? t("wsOnline") : t("wsReconnecting");
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
        toast(t("toastFileReceived", msg.text));
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
    confirmText = t("confirm"),
    cancelText = t("cancel"),
    inputValue = "",
    showInput = false,
    danger = false,
  }) {
    $("modalTitle").textContent = title || t("confirm");
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
    const name = state.selected === ALL ? t("allConversation") : peerLabel(state.selected);
    const result = await showDialog({
      title: t("clearTitle"),
      message: t("clearMessage", name),
      confirmText: t("delete"),
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/history/" + encodeURIComponent(state.selected), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast(t("toastClearFailed", data && data.error ? data.error : res.status));
        return;
      }
      removeConversation(state.selected);
      toast(t("toastCleared"));
    } catch (err) {
      toast(t("toastClearError", err.message));
    }
  }

  async function deleteMessage(entry) {
    if (!entry || !entry.id) return;
    const result = await showDialog({
      title: t("deleteMessageTitle"),
      message: t("deleteMessageBody"),
      confirmText: t("delete"),
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      const res = await fetch("/api/history/entry/" + encodeURIComponent(entry.id), { method: "DELETE" });
      const data = await jsonOrNull(res);
      if (!res.ok) {
        toast(t("toastMessageDeleteFailed", data && data.error ? data.error : res.status));
        return;
      }
      removeEntry((data && data.entry) || entry);
      toast(t("toastMessageDeleted"));
    } catch (err) {
      toast(t("toastMessageDeleteError", err.message));
    }
  }

  async function shutdown() {
    const result = await showDialog({
      title: t("shutdownTitle"),
      message: t("shutdownBody"),
      confirmText: t("exit"),
      danger: true,
    });
    if (!result.confirmed) return;
    try {
      await fetch("/api/shutdown", { method: "POST" });
    } catch {}
    $("bye").hidden = false;
  }

  async function showHelp() {
    if (!$("modalLayer").hidden) return;
    await showDialog({
      title: t("helpTitle"),
      message: t("helpBody"),
      confirmText: t("close"),
      cancelText: "",
    });
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
    applyLanguage(state.lang);
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
    $("langBtn").addEventListener("click", toggleLanguage);
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
    document.addEventListener("keydown", (e) => {
      if (e.key === "F1") {
        e.preventDefault();
        showHelp();
      }
    });

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
