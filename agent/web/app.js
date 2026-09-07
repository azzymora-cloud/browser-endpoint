const loginView = document.getElementById("loginView");
const appView = document.getElementById("appView");
const loginError = document.getElementById("loginError");
const hostLabel = document.getElementById("hostLabel");

let token = sessionStorage.getItem("thinkcentre_token") || "";

async function api(path, opts = {}) {
  const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  if (token) headers.Authorization = "Bearer " + token;
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) {
    token = "";
    sessionStorage.removeItem("thinkcentre_token");
    showLogin("Session expired. Enter the agent token again.");
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    throw new Error(data.error || res.statusText);
  }
  return data;
}

function showLogin(msg) {
  loginView.hidden = false;
  appView.hidden = true;
  hostLabel.textContent = "Locked";
  loginError.hidden = !msg;
  loginError.textContent = msg || "";
}

function showApp() {
  loginView.hidden = true;
  appView.hidden = false;
}

document.getElementById("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  token = document.getElementById("token").value.trim();
  try {
    await api("/api/login", { method: "POST", body: JSON.stringify({ token }) });
    sessionStorage.setItem("thinkcentre_token", token);
    await refresh();
  } catch (err) {
    showLogin(err.message || "Wrong token");
  }
});

document.getElementById("stackUp").addEventListener("click", () => runAction("/api/stack/up"));
document.getElementById("stackDown").addEventListener("click", () => runAction("/api/stack/down"));
document.getElementById("enableWol").addEventListener("click", () => runAction("/api/wol/enable", "wolOut"));
document.getElementById("wakeAll").addEventListener("click", () => runAction("/api/wake", "wolOut"));
document.getElementById("cancelReboot").addEventListener("click", () => runAction("/api/reboot/cancel", "rebootMsg"));

document.getElementById("rebootBtn").addEventListener("click", async () => {
  const ok = window.confirm("Reboot this ThinkCentre now? Remote desktop will drop until the PC and tunnel are back.");
  if (!ok) return;
  try {
    const data = await api("/api/reboot", { method: "POST", body: JSON.stringify({ confirm: "reboot" }) });
    const el = document.getElementById("rebootMsg");
    el.hidden = false;
    el.textContent = data.message + " in " + data.seconds + "s.";
  } catch (err) {
    const el = document.getElementById("rebootMsg");
    el.hidden = false;
    el.textContent = err.message;
  }
});

async function runAction(path, logId) {
  try {
    const data = await api(path, { method: "POST", body: "{}" });
    if (logId) {
      const el = document.getElementById(logId);
      el.hidden = false;
      el.textContent = data.output || JSON.stringify(data, null, 2);
    }
    await refresh();
  } catch (err) {
    if (logId) {
      const el = document.getElementById(logId);
      el.hidden = false;
      el.textContent = err.message;
    } else {
      alert(err.message);
    }
  }
}

function renderStatus(s) {
  hostLabel.textContent = s.hostname + " · " + s.uptime;
  document.getElementById("machineFacts").innerHTML =
    fact("Hostname", s.hostname) +
    fact("OS", s.os + "/" + s.arch) +
    fact("Agent listen", s.listen) +
    fact("Stack folder", s.stackDir) +
    fact("Reboot delay", s.rebootSec + "s") +
    fact("Reboot from this UI", s.canReboot ? "Yes (Windows)" : "Not on this OS");

  document.getElementById("wolNote").textContent = s.wolNote || "";
  const tb = document.getElementById("adapterBody");
  tb.innerHTML = "";
  (s.adapters || []).forEach((a) => {
    const tr = document.createElement("tr");
    tr.innerHTML =
      "<td>" + escapeHtml(a.name) + (a.up ? " <span class=\"dot on\"></span>" : " <span class=\"dot off\"></span>") + "</td>" +
      "<td class=\"mac\">" + escapeHtml(a.mac) + "</td>" +
      "<td>" + escapeHtml(a.addrs || "—") + "</td>" +
      "<td></td>";
    tr.lastElementChild.appendChild(wakeButton(a.mac));
    tb.appendChild(tr);
  });

  const stack = s.stack || {};
  const list = document.getElementById("stackList");
  list.innerHTML = "";
  if (!stack.services || !stack.services.length) {
    const li = document.createElement("li");
    li.textContent = stack.error || "No compose services reported. Start the stack.";
    list.appendChild(li);
  } else {
    stack.services.forEach((svc) => {
      const li = document.createElement("li");
      const mark = document.createElement("span");
      mark.innerHTML = "<span class=\"dot " + (svc.running ? "on" : "off") + "\"></span>" + escapeHtml(svc.name || "?");
      const st = document.createElement("span");
      st.textContent = [svc.state, svc.health, svc.status].filter(Boolean).join(" · ");
      li.appendChild(mark);
      li.appendChild(st);
      list.appendChild(li);
    });
  }
  const err = document.getElementById("stackError");
  if (stack.error && stack.services && stack.services.length) {
    err.hidden = false;
    err.textContent = stack.error;
  } else {
    err.hidden = true;
  }
}

function wakeButton(mac) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "ghost";
  btn.textContent = "Packet";
  btn.addEventListener("click", async () => {
    try {
      const data = await api("/api/wake", { method: "POST", body: JSON.stringify({ mac }) });
      const el = document.getElementById("wolOut");
      el.hidden = false;
      el.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      const el = document.getElementById("wolOut");
      el.hidden = false;
      el.textContent = err.message;
    }
  });
  return btn;
}

function fact(k, v) {
  return "<dt>" + escapeHtml(k) + "</dt><dd>" + escapeHtml(String(v ?? "—")) + "</dd>";
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

async function refresh() {
  const s = await api("/api/status");
  showApp();
  renderStatus(s);
}

if (token) {
  refresh().catch(() => showLogin(""));
} else {
  showLogin("");
}
