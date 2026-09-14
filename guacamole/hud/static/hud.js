(function () {
  "use strict";

  var KEY_CTRL = 0xFFE3;
  var KEY_ALT = 0xFFE9;
  var KEY_DEL = 0xFFFF;

  var hud = null;
  var aimCursor = null;
  var immersive = false;
  var exiting = false;
  var arming = false;
  var viewX = 0;
  var viewY = 0;
  var haveMouse = false;
  var buttons = { left: false, middle: false, right: false, up: false, down: false };
  var toastTimer = 0;
  var aimedButton = null;

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

  function displayElement() {
    var client = findGuacClient();
    if (client && client.getDisplay) {
      var el = client.getDisplay().getElement();
      if (el) return el;
    }
    return document.querySelector(".display");
  }

  function displayBox() {
    var el = displayElement();
    return el ? el.getBoundingClientRect() : { left: 0, top: 0, width: window.innerWidth, height: window.innerHeight };
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

  function mouseStateFromView() {
    var G = window.Guacamole;
    if (!G || !G.Mouse || !G.Mouse.State) return null;
    var box = displayBox();
    var x = Math.max(0, Math.min(Math.max(0, box.width - 1), viewX - box.left));
    var y = Math.max(0, Math.min(Math.max(0, box.height - 1), viewY - box.top));
    return new G.Mouse.State({
      x: x,
      y: y,
      left: buttons.left,
      middle: buttons.middle,
      right: buttons.right,
      up: buttons.up,
      down: buttons.down
    });
  }

  function sendMouse() {
    var client = findGuacClient();
    var state = mouseStateFromView();
    if (!client || !state) return;
    client.sendMouseState(state, true);
  }

  function toast(msg) {
    if (!hud) return;
    var el = hud.querySelector(".hud-toast");
    el.hidden = false;
    el.textContent = msg;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.hidden = true; }, 4200);
  }

  function pointInRect(x, y, rect, pad) {
    pad = pad || 0;
    return x >= rect.left - pad && x <= rect.right + pad &&
      y >= rect.top - pad && y <= rect.bottom + pad;
  }

  function hudShellRect() {
    if (!hud) return null;
    var shell = hud.querySelector(".hud-shell");
    return shell ? shell.getBoundingClientRect() : null;
  }

  function visibleHudButtons() {
    if (!hud) return [];
    return Array.prototype.filter.call(hud.querySelectorAll("button"), function (btn) {
      var r = btn.getBoundingClientRect();
      return r.width > 0 && r.height > 0;
    });
  }

  function hudButtonAt(x, y) {
    var list = visibleHudButtons();
    for (var i = 0; i < list.length; i++) {
      if (pointInRect(x, y, list[i].getBoundingClientRect(), 2)) return list[i];
    }
    return null;
  }

  function overHud(x, y) {
    var shell = hudShellRect();
    return !!(shell && pointInRect(x, y, shell, 6));
  }

  function nearHud(x, y) {
    var shell = hudShellRect();
    return !!(shell && pointInRect(x, y, shell, 48));
  }

  function setAimedButton(btn) {
    if (aimedButton === btn) return;
    if (aimedButton) aimedButton.classList.remove("aim");
    aimedButton = btn;
    if (aimedButton) aimedButton.classList.add("aim");
  }

  function updateAimCursor() {
    if (!aimCursor) return;
    var show = immersive && document.pointerLockElement && nearHud(viewX, viewY);
    aimCursor.hidden = !show;
    if (!show) {
      setAimedButton(null);
      return;
    }
    aimCursor.style.transform = "translate(" + viewX + "px, " + viewY + "px)";
    setAimedButton(hudButtonAt(viewX, viewY));
  }

  function clampView(dx, dy) {
    viewX = Math.max(0, Math.min(window.innerWidth - 1, viewX + dx));
    viewY = Math.max(0, Math.min(window.innerHeight - 1, viewY + dy));
  }

  function syncHudState() {
    if (!hud) return;
    hud.classList.toggle("immersive", immersive);
    if (immersive) hud.classList.add("open");
    var btn = hud.querySelector("[data-action=immersive]");
    if (btn) {
      btn.classList.toggle("active", immersive);
      btn.setAttribute("aria-pressed", immersive ? "true" : "false");
    }
    if (!immersive) {
      setAimedButton(null);
      if (aimCursor) aimCursor.hidden = true;
    }
  }

  function lockPointer() {
    var el = displayElement() || document.documentElement;
    if (!el || typeof el.requestPointerLock !== "function") return;
    try {
      var result = el.requestPointerLock({ unadjustedMovement: true });
      if (result && typeof result.catch === "function") {
        result.catch(function () { el.requestPointerLock(); });
      }
    } catch (err) {
      el.requestPointerLock();
    }
  }

  function seedViewFromLastMouse() {
    if (haveMouse) return;
    var box = displayBox();
    viewX = box.left + box.width / 2;
    viewY = box.top + box.height / 2;
    haveMouse = true;
  }

  function enterImmersive() {
    if (immersive) return;
    if (!findGuacClient()) {
      toast("Connect a desktop first");
      return;
    }

    immersive = true;
    arming = true;
    syncHudState();
    seedViewFromLastMouse();
    lockPointer();
    toast("Immersive on — mouse stays in the stream. Move up to P to exit. Ctrl+Alt+I also works");
  }

  function exitImmersive(msg) {
    if (!immersive && !document.pointerLockElement) {
      syncHudState();
      return;
    }
    immersive = false;
    exiting = true;
    arming = false;
    buttons.left = buttons.middle = buttons.right = false;
    try { sendMouse(); } catch (err) { /* session may already be gone */ }
    if (document.pointerLockElement) document.exitPointerLock();
    syncHudState();
    exiting = false;
    if (msg) toast(msg);
  }

  function setImmersive(on) {
    if (on) enterImmersive();
    else exitImmersive("Immersive mode off");
  }

  function onPointerLockChange() {
    if (exiting) return;
    if (arming && document.pointerLockElement) {
      arming = false;
      return;
    }
    if (arming) return;
    if (immersive && !document.pointerLockElement) {
      exitImmersive("Immersive mode off");
    }
  }

  function onPointerLockError() {
    arming = false;
    if (!immersive) return;
    immersive = false;
    syncHudState();
    toast("Could not lock the mouse — click the desktop and try Immersive again");
  }

  function isToggleHotkey(ev) {
    return ev.ctrlKey && ev.altKey && !ev.shiftKey && !ev.metaKey &&
      (ev.code === "KeyI" || ev.key === "i" || ev.key === "I");
  }

  function onKeyDown(ev) {
    if (!onClientPage()) return;
    if (!isToggleHotkey(ev)) return;
    ev.preventDefault();
    ev.stopImmediatePropagation();
    setImmersive(!immersive);
  }

  function trackUnlockedMouse(ev) {
    if (document.pointerLockElement) return;
    viewX = ev.clientX;
    viewY = ev.clientY;
    haveMouse = true;
  }

  function dispatchHudClick(btn) {
    if (!btn) return;
    btn.click();
  }

  function onLockedInput(ev) {
    if (!immersive || !document.pointerLockElement) return;
    ev.preventDefault();
    ev.stopImmediatePropagation();

    if (ev.type === "contextmenu") return;

    if (ev.type === "mousemove") {
      clampView(ev.movementX || 0, ev.movementY || 0);
      updateAimCursor();
      if (overHud(viewX, viewY)) return;
      sendMouse();
      return;
    }

    if (ev.type === "mousedown" || ev.type === "mouseup") {
      var hudBtn = hudButtonAt(viewX, viewY);
      if (hudBtn || overHud(viewX, viewY)) {
        if (ev.type === "mousedown" && ev.button === 0) dispatchHudClick(hudBtn);
        return;
      }
      var down = ev.type === "mousedown";
      if (ev.button === 0) buttons.left = down;
      if (ev.button === 1) buttons.middle = down;
      if (ev.button === 2) buttons.right = down;
      sendMouse();
      return;
    }

    if (ev.type === "wheel") {
      if (overHud(viewX, viewY)) return;
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
          '<button type="button" data-action="immersive" aria-pressed="false" title="Lock mouse to the stream (Ctrl+Alt+I)">Immersive</button>' +
          '<button type="button" class="danger" data-action="kill">Kill focused app</button>' +
        '</div>' +
      '</div>' +
      '<div class="hud-toast" hidden></div>';
    document.body.appendChild(hud);

    aimCursor = document.createElement("div");
    aimCursor.id = "session-hud-aim";
    aimCursor.hidden = true;
    document.body.appendChild(aimCursor);

    hud.querySelector(".hud-toggle").addEventListener("click", function () {
      if (immersive) {
        setImmersive(false);
        return;
      }
      hud.classList.toggle("open");
    });
    hud.querySelector("[data-action=cad]").addEventListener("click", sendCAD);
    hud.querySelector("[data-action=immersive]").addEventListener("click", function () {
      setImmersive(!immersive);
    });
    hud.querySelector("[data-action=kill]").addEventListener("click", killForeground);

    document.addEventListener("mousemove", trackUnlockedMouse, true);
    ["mousemove", "mousedown", "mouseup", "wheel", "contextmenu"].forEach(function (type) {
      document.addEventListener(type, onLockedInput, true);
    });
    document.addEventListener("keydown", onKeyDown, true);
    document.addEventListener("pointerlockchange", onPointerLockChange);
    document.addEventListener("pointerlockerror", onPointerLockError);
    return hud;
  }

  function syncVisibility() {
    ensureHud();
    var show = onClientPage();
    hud.hidden = !show;
    if (!show) {
      hud.classList.remove("open");
      exitImmersive();
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
