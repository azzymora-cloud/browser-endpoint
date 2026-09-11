(function () {
  "use strict";

  var KEY_CTRL = 0xFFE3;
  var KEY_ALT = 0xFFE9;
  var KEY_DEL = 0xFFFF;

  var hud = null;
  var lockLayer = null;
  var captive = false;
  var mouseX = 0;
  var mouseY = 0;
  var buttons = { left: false, middle: false, right: false, up: false, down: false };
  var toastTimer = 0;

  function onClientPage() {
    return /#\/client\//.test(location.hash || "");
  }

  function findGuacClient() {
    if (typeof window.angular === "undefined") return null;
    var seeds = document.querySelectorAll(".display, .client-main, .client-view, guac-client");
    for (var i = 0; i < seeds.length; i++) {
      try {
        var scope = window.angular.element(seeds[i]).scope();
        while (scope) {
          var managed = scope.client || scope.focusedClient;
          if (managed && managed.client && typeof managed.client.sendKeyEvent === "function") {
            return managed.client;
          }
          scope = scope.$parent;
        }
      } catch (err) { /* keep walking */ }
    }
    return null;
  }

  function displayBox() {
    var client = findGuacClient();
    if (client && client.getDisplay) {
      var el = client.getDisplay().getElement();
      if (el) return el.getBoundingClientRect();
    }
    var fallback = document.querySelector(".display");
    return fallback ? fallback.getBoundingClientRect() : { left: 0, top: 0, width: window.innerWidth, height: window.innerHeight };
  }

  function sendKeys(down, keysyms) {
    var client = findGuacClient();
    if (!client) return false;
    for (var i = 0; i < keysyms.length; i++) client.sendKeyEvent(down ? 1 : 0, keysyms[i]);
    return true;
  }

  function sendCAD() {
    var down = [KEY_CTRL, KEY_ALT, KEY_DEL];
    if (!sendKeys(true, down)) {
      toast("Connect a desktop first");
      return;
    }
    setTimeout(function () {
      sendKeys(false, [KEY_DEL, KEY_ALT, KEY_CTRL]);
      toast("Sent Ctrl+Alt+Del");
    }, 40);
  }

  function mouseState() {
    var G = window.Guacamole;
    if (!G || !G.Mouse || !G.Mouse.State) return null;
    return new G.Mouse.State({
      x: mouseX,
      y: mouseY,
      left: buttons.left,
      middle: buttons.middle,
      right: buttons.right,
      up: buttons.up,
      down: buttons.down
    });
  }

  function sendMouse() {
    var client = findGuacClient();
    var state = mouseState();
    if (!client || !state) return;
    client.sendMouseState(state, true);
  }

  function clampMouse(dx, dy) {
    var box = displayBox();
    mouseX = Math.max(0, Math.min(box.width - 1, mouseX + dx));
    mouseY = Math.max(0, Math.min(box.height - 1, mouseY + dy));
  }

  function toast(msg) {
    if (!hud) return;
    var el = hud.querySelector(".hud-toast");
    el.hidden = false;
    el.textContent = msg;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.hidden = true; }, 3200);
  }

  function setCaptive(on) {
    captive = !!on;
    var btn = hud && hud.querySelector("[data-action=captive]");
    if (btn) btn.classList.toggle("active", captive);
    if (!lockLayer) return;
    if (!captive) {
      lockLayer.hidden = true;
      if (document.pointerLockElement) document.exitPointerLock();
      buttons.left = buttons.middle = buttons.right = false;
      return;
    }
    var box = displayBox();
    mouseX = box.width / 2;
    mouseY = box.height / 2;
    lockLayer.hidden = false;
    lockLayer.requestPointerLock();
    toast("Captive mouse on — Esc to release");
  }

  function onLockChange() {
    if (!captive) return;
    if (!document.pointerLockElement) {
      setCaptive(false);
      toast("Captive mouse off");
    }
  }

  function onLockMouse(ev) {
    if (!captive) return;
    ev.preventDefault();
    ev.stopPropagation();
    if (ev.type === "mousemove") {
      clampMouse(ev.movementX || 0, ev.movementY || 0);
      sendMouse();
      return;
    }
    if (ev.type === "mousedown" || ev.type === "mouseup") {
      var down = ev.type === "mousedown";
      if (ev.button === 0) buttons.left = down;
      if (ev.button === 1) buttons.middle = down;
      if (ev.button === 2) buttons.right = down;
      sendMouse();
      return;
    }
    if (ev.type === "wheel") {
      var wheelDown = ev.deltaY > 0;
      buttons.up = !wheelDown;
      buttons.down = wheelDown;
      sendMouse();
      buttons.up = buttons.down = false;
      sendMouse();
    }
  }

  function confirmKill(info) {
    var name = (info && (info.title || info.name)) || "the app in focus";
    return window.confirm(
      "Force-stop " + name + " on the PC?\n\n" +
      "Use this when a window is frozen and blocking the desktop. Explorer and other Windows shell processes are protected."
    );
  }

  function killForeground() {
    fetch("/host-agent/foreground")
      .then(function (res) { return res.json().catch(function () { return {}; }); })
      .then(function (info) {
        if (info && info.error && !info.ok) {
          toast(info.error);
          return;
        }
        if (!confirmKill(info && info.ok ? info : null)) return;
        return fetch("/host-agent/kill-foreground", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ confirm: "kill" })
        }).then(function (res) { return res.json().then(function (data) { return { res: res, data: data }; }); });
      })
      .then(function (result) {
        if (!result) return;
        if (!result.res.ok || !result.data.ok) {
          toast(result.data.error || "Kill failed");
          return;
        }
        toast("Stopped " + (result.data.name || "app"));
      })
      .catch(function () {
        toast("Kill switch unreachable — is the agent running?");
      });
  }

  function ensureHud() {
    if (hud) return hud;
    var link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/session-hud/hud.css";
    document.head.appendChild(link);

    hud = document.createElement("div");
    hud.id = "session-hud";
    hud.hidden = true;
    hud.innerHTML =
      '<div class="hud-shell">' +
        '<button type="button" class="hud-toggle" title="Session controls">P</button>' +
        '<div class="hud-actions">' +
          '<button type="button" data-action="cad">Ctrl+Alt+Del</button>' +
          '<button type="button" data-action="captive">Captive mouse</button>' +
          '<button type="button" class="danger" data-action="kill">Kill focused app</button>' +
        '</div>' +
      '</div>' +
      '<div class="hud-toast" hidden></div>';
    document.body.appendChild(hud);

    lockLayer = document.createElement("div");
    lockLayer.id = "session-hud-lock";
    lockLayer.hidden = true;
    document.body.appendChild(lockLayer);

    hud.querySelector(".hud-toggle").addEventListener("click", function () {
      hud.classList.toggle("open");
    });
    hud.querySelector("[data-action=cad]").addEventListener("click", sendCAD);
    hud.querySelector("[data-action=captive]").addEventListener("click", function () {
      setCaptive(!captive);
    });
    hud.querySelector("[data-action=kill]").addEventListener("click", killForeground);

    ["mousemove", "mousedown", "mouseup", "wheel", "contextmenu"].forEach(function (type) {
      lockLayer.addEventListener(type, function (ev) {
        if (type === "contextmenu") ev.preventDefault();
        onLockMouse(ev);
      });
    });
    document.addEventListener("pointerlockchange", onLockChange);
    return hud;
  }

  function syncVisibility() {
    ensureHud();
    var show = onClientPage();
    hud.hidden = !show;
    if (!show) {
      hud.classList.remove("open");
      setCaptive(false);
    }
  }

  function boot() {
    syncVisibility();
    window.addEventListener("hashchange", syncVisibility);
    setInterval(syncVisibility, 1500);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
