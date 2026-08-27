const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

const state = {
  menus: [],
  isos: [],
  settings: null,
  currentMenuId: null,
};

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  if (res.status === 204) return null;
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) return res.json();
  return res.text();
}

function fmtSize(n) {
  if (n < 1024) return n + " B";
  if (n < 1024 ** 2) return (n / 1024).toFixed(1) + " KB";
  if (n < 1024 ** 3) return (n / 1024 ** 2).toFixed(1) + " MB";
  return (n / 1024 ** 3).toFixed(2) + " GB";
}

function setMsg(el, text, ok = true) {
  el.textContent = text || "";
  el.className = "msg " + (ok ? "ok" : "err");
}

/* tabs */
$$(".tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    $$(".tab").forEach((b) => b.classList.remove("active"));
    $$(".panel").forEach((p) => p.classList.remove("active"));
    btn.classList.add("active");
    $("#tab-" + btn.dataset.tab).classList.add("active");
    if (btn.dataset.tab === "export") refreshEmbedPreview();
  });
});

/* settings */
function fillSettings(s) {
  const f = $("#settings-form");
  f.serverName.value = s.serverName || "";
  f.publicUrl.value = s.publicUrl || "";
  f.chainUrl.value = s.chainUrl || "";
  f.defaultMode.value = s.defaultNetwork?.mode || "dhcp";
  f.defaultIp.value = s.defaultNetwork?.ip || "";
  f.defaultNetmask.value = s.defaultNetwork?.netmask || "";
  f.defaultGateway.value = s.defaultNetwork?.gateway || "";
  f.defaultDns.value = s.defaultNetwork?.dns || "";
  f.isoMode.value = s.isoNetwork?.mode || "dhcp";
  f.isoIp.value = s.isoNetwork?.ip || "";
  f.isoNetmask.value = s.isoNetwork?.netmask || "";
  f.isoGateway.value = s.isoNetwork?.gateway || "";
  f.isoDns.value = s.isoNetwork?.dns || "";
}

$("#settings-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const f = e.target;
  const body = {
    serverName: f.serverName.value.trim(),
    publicUrl: f.publicUrl.value.trim(),
    chainUrl: f.chainUrl.value.trim(),
    defaultNetwork: {
      mode: f.defaultMode.value,
      ip: f.defaultIp.value.trim(),
      netmask: f.defaultNetmask.value.trim(),
      gateway: f.defaultGateway.value.trim(),
      dns: f.defaultDns.value.trim(),
    },
    isoNetwork: {
      mode: f.isoMode.value,
      ip: f.isoIp.value.trim(),
      netmask: f.isoNetmask.value.trim(),
      gateway: f.isoGateway.value.trim(),
      dns: f.isoDns.value.trim(),
    },
  };
  try {
    state.settings = await api("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    setMsg($("#settings-msg"), "已保存");
  } catch (err) {
    setMsg($("#settings-msg"), err.message, false);
  }
});

/* menus */
function renderMenuList() {
  const ul = $("#menu-list");
  ul.innerHTML = "";
  state.menus.forEach((m) => {
    const li = document.createElement("li");
    const btn = document.createElement("button");
    btn.type = "button";
    btn.textContent = m.name;
    if (m.id === state.currentMenuId) btn.classList.add("active");
    btn.addEventListener("click", () => selectMenu(m.id));
    li.appendChild(btn);
    ul.appendChild(li);
  });
}

function itemCard(item = {}) {
  const wrap = document.createElement("div");
  wrap.className = "item-card";
  wrap.innerHTML = `
    <label>ID <input name="id" value="${esc(item.id || "")}" /></label>
    <label>标签 <input name="label" value="${esc(item.label || "")}" /></label>
    <label>类型
      <select name="type">
        ${["chain","kernel","sanboot","iso","shell","exit","custom"].map((t) =>
          `<option value="${t}" ${item.type === t ? "selected" : ""}>${t}</option>`).join("")}
      </select>
    </label>
    <label>启用
      <select name="enabled">
        <option value="true" ${item.enabled !== false ? "selected" : ""}>是</option>
        <option value="false" ${item.enabled === false ? "selected" : ""}>否</option>
      </select>
    </label>
    <label>URL <input name="url" value="${esc(item.url || "")}" /></label>
    <label>Kernel <input name="kernel" value="${esc(item.kernel || "")}" /></label>
    <label>Initrd <input name="initrd" value="${esc(item.initrd || "")}" /></label>
    <label>Args <input name="args" value="${esc(item.args || "")}" /></label>
    <label>ISO
      <select name="isoId">
        <option value="">—</option>
        ${state.isos.map((iso) =>
          `<option value="${esc(iso.id)}" ${item.isoId === iso.id ? "selected" : ""}>${esc(iso.name)}</option>`
        ).join("")}
      </select>
    </label>
    <label class="full">Custom 脚本
      <textarea name="custom" rows="3">${esc(item.custom || "")}</textarea>
    </label>
    <div class="item-actions">
      <button type="button" class="btn small danger btn-rm">移除</button>
    </div>
  `;
  $(".btn-rm", wrap).addEventListener("click", () => wrap.remove());
  return wrap;
}

function esc(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function selectMenu(id) {
  const m = state.menus.find((x) => x.id === id);
  if (!m) return;
  state.currentMenuId = id;
  renderMenuList();
  const f = $("#menu-form");
  f.id.value = m.id;
  f.name.value = m.name || "";
  f.description.value = m.description || "";
  f.timeoutSec.value = m.timeoutSec ?? 30;
  f.defaultItem.value = m.defaultItem || "";
  f.rawScript.value = m.rawScript || "";
  const box = $("#items");
  box.innerHTML = "";
  (m.items || []).forEach((it) => box.appendChild(itemCard(it)));
  $("#preview").hidden = true;
}

function collectMenu() {
  const f = $("#menu-form");
  const items = $$(".item-card", $("#items")).map((card) => ({
    id: $("[name=id]", card).value.trim(),
    label: $("[name=label]", card).value.trim(),
    type: $("[name=type]", card).value,
    enabled: $("[name=enabled]", card).value === "true",
    url: $("[name=url]", card).value.trim(),
    kernel: $("[name=kernel]", card).value.trim(),
    initrd: $("[name=initrd]", card).value.trim(),
    args: $("[name=args]", card).value.trim(),
    isoId: $("[name=isoId]", card).value,
    custom: $("[name=custom]", card).value,
  }));
  return {
    id: f.id.value,
    name: f.name.value.trim(),
    description: f.description.value.trim(),
    timeoutSec: Number(f.timeoutSec.value || 30),
    defaultItem: f.defaultItem.value.trim(),
    rawScript: f.rawScript.value,
    items,
  };
}

$("#btn-add-item").addEventListener("click", () => {
  $("#items").appendChild(itemCard({ type: "chain", enabled: true }));
});

$("#btn-new-menu").addEventListener("click", () => {
  state.currentMenuId = null;
  renderMenuList();
  const f = $("#menu-form");
  f.reset();
  f.id.value = "";
  $("#items").innerHTML = "";
  $("#items").appendChild(itemCard({ id: "shell", label: "iPXE Shell", type: "shell", enabled: true }));
});

$("#menu-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const body = collectMenu();
  try {
    let saved;
    if (body.id) {
      saved = await api("/api/menus/" + body.id, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    } else {
      saved = await api("/api/menus", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    }
    await loadMenus();
    selectMenu(saved.id);
    setMsg($("#menu-msg"), "菜单已保存");
  } catch (err) {
    setMsg($("#menu-msg"), err.message, false);
  }
});

$("#btn-delete-menu").addEventListener("click", async () => {
  const id = $("#menu-form").id.value;
  if (!id) return;
  if (!confirm("删除菜单 " + id + " ?")) return;
  try {
    await api("/api/menus/" + id, { method: "DELETE" });
    await loadMenus();
    if (state.menus[0]) selectMenu(state.menus[0].id);
    setMsg($("#menu-msg"), "已删除");
  } catch (err) {
    setMsg($("#menu-msg"), err.message, false);
  }
});

$("#btn-preview").addEventListener("click", async () => {
  const id = $("#menu-form").id.value;
  if (!id) {
    setMsg($("#menu-msg"), "请先保存菜单再预览", false);
    return;
  }
  try {
    const text = await api("/api/menus/" + id + "/preview");
    const pre = $("#preview");
    pre.hidden = false;
    pre.textContent = text;
  } catch (err) {
    setMsg($("#menu-msg"), err.message, false);
  }
});

async function loadMenus() {
  state.menus = await api("/api/menus");
  renderMenuList();
}

/* isos */
function renderISOs() {
  const tb = $("#iso-body");
  tb.innerHTML = "";
  state.isos.forEach((iso) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${esc(iso.name)}</td>
      <td><a class="link" href="/files/isos/${encodeURIComponent(iso.filename)}" target="_blank">${esc(iso.filename)}</a></td>
      <td>${fmtSize(iso.size)}</td>
      <td>${esc(iso.uploadedAt || "")}</td>
      <td><button type="button" class="btn small danger">删除</button></td>
    `;
    $("button", tr).addEventListener("click", async () => {
      if (!confirm("删除 " + iso.name + " ?")) return;
      try {
        await api("/api/isos/" + iso.id, { method: "DELETE" });
        await loadISOs();
      } catch (err) {
        setMsg($("#iso-msg"), err.message, false);
      }
    });
    tb.appendChild(tr);
  });
}

$("#iso-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const f = e.target;
  const fd = new FormData();
  fd.append("file", f.file.files[0]);
  if (f.name.value.trim()) fd.append("name", f.name.value.trim());
  if (f.note.value.trim()) fd.append("note", f.note.value.trim());
  setMsg($("#iso-msg"), "上传中…");
  try {
    await api("/api/isos", { method: "POST", body: fd });
    f.reset();
    await loadISOs();
    setMsg($("#iso-msg"), "上传完成");
  } catch (err) {
    setMsg($("#iso-msg"), err.message, false);
  }
});

async function loadISOs() {
  state.isos = await api("/api/isos");
  renderISOs();
  if (state.currentMenuId) selectMenu(state.currentMenuId);
}

async function refreshEmbedPreview() {
  try {
    $("#embed-preview").textContent = await api("/api/iso/embed.ipxe");
  } catch (err) {
    $("#embed-preview").textContent = err.message;
  }
}

async function boot() {
  try {
    const h = await api("/api/health");
    const el = $("#health");
    el.textContent = `v${h.version}` + (h.assets ? " · assets ok" : " · assets 缺失");
    el.classList.add("ok");
  } catch {
    $("#health").textContent = "API 不可用";
  }
  state.settings = await api("/api/settings");
  fillSettings(state.settings);
  await loadISOs();
  await loadMenus();
  if (state.menus[0]) selectMenu(state.menus[0].id);
  await refreshEmbedPreview();
}

boot().catch((err) => {
  console.error(err);
  setMsg($("#settings-msg"), err.message, false);
});
