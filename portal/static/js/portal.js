/* Litefunctions portal — shell interactions, command palette, small UI helpers. */
(function () {
  "use strict";

  const navItems = [
    { group: "Workspace", label: "Overview", href: "/dashboard/", icon: "grid", keys: "g o" },
    { group: "Build", label: "Functions", href: "/functions/", icon: "code", keys: "g f" },
    { group: "Build", label: "Plan", href: "/agent/", icon: "sparkles" },
    { group: "Data", label: "Objects", href: "/data/", icon: "box" },
    { group: "Data", label: "Tables", href: "/data-model/", icon: "table" },
    { group: "Data", label: "Queries", href: "/queries/", icon: "braces" },
    { group: "Project", label: "Dependencies", href: "/dependencies/", icon: "package" },
    { group: "Project", label: "Configs & Secrets", href: "/configuration/", icon: "key" },
    { group: "Project", label: "Team & Access", href: "/access/", icon: "users" }
  ];

  const iconPaths = {
    grid: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
    code: '<path d="m8 6-6 6 6 6"/><path d="m16 6 6 6-6 6"/>',
    file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/>',
    box: '<path d="m21 8-9-5-9 5 9 5 9-5Z"/><path d="M3 8v8l9 5 9-5V8"/><path d="M12 13v8"/>',
    table: '<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18"/>',
    braces: '<path d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5a2 2 0 0 0 2 2h1"/><path d="M16 21h1a2 2 0 0 0 2-2v-5a2 2 0 0 1 2-2 2 2 0 0 1-2-2V5a2 2 0 0 0-2-2h-1"/>',
    "chevron-right": '<path d="m9 18 6-6-6-6"/>',
    "chevron-down": '<path d="m6 9 6 6 6-6"/>',
    route: '<circle cx="6" cy="19" r="3"/><path d="M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15"/><circle cx="18" cy="5" r="3"/>',
    package: '<path d="m7.5 4.27 9 5.15"/><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/>',
    database: '<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/>',
    sparkles: '<path d="m12 3-1.9 5.8a2 2 0 0 1-1.3 1.3L3 12l5.8 1.9a2 2 0 0 1 1.3 1.3L12 21l1.9-5.8a2 2 0 0 1 1.3-1.3L21 12l-5.8-1.9a2 2 0 0 1-1.3-1.3Z"/>',
    layers: '<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>',
    network: '<rect x="16" y="16" width="6" height="6" rx="1"/><rect x="2" y="16" width="6" height="6" rx="1"/><rect x="9" y="2" width="6" height="6" rx="1"/><path d="M5 16v-3a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v3"/><path d="M12 12V8"/>',
    shield: '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1Z"/>',
    cpu: '<rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M15 2v2M15 20v2M2 15h2M2 9h2M20 15h2M20 9h2M9 2v2M9 20v2"/>',
    activity: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
    rocket: '<path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/>',
    key: '<circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6M15.5 7.5l3 3L22 7l-3-3"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
    clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>'
  };

  window.zIcon = function (name, cls) {
    const p = iconPaths[name] || iconPaths.grid;
    return '<svg class="' + (cls || "") + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">' + p + "</svg>";
  };

  // ---------- toasts ----------
  window.toast = function (msg, type) {
    type = type || "info";
    let container = document.getElementById("toast-container");
    if (!container) {
      container = document.createElement("div");
      container.id = "toast-container";
      document.body.appendChild(container);
    }
    const icons = {
      success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="m5 13 4 4L19 7"/></svg>',
      error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M18 6 6 18M6 6l12 12"/></svg>',
      info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path d="M12 8h.01M12 11v5"/></svg>'
    };
    const t = document.createElement("div");
    t.className = "z-toast " + type;
    t.innerHTML = (icons[type] || icons.info) + "<span></span>";
    t.querySelector("span").textContent = msg;
    const close = () => { t.classList.add("out"); setTimeout(() => t.remove(), 200); };
    t.addEventListener("click", close);
    container.appendChild(t);
    setTimeout(close, 3800);
  };

  // ---------- sidebar / user menu ----------
  function applySidebarState(collapsed) {
    const app = document.querySelector(".z-app");
    if (app) app.classList.toggle("sidebar-collapsed", collapsed);
    const btn = document.querySelector("[data-sidebar-collapse]");
    if (btn) {
      const label = collapsed ? "Expand sidebar" : "Collapse sidebar";
      btn.title = label;
      btn.setAttribute("aria-label", label);
    }
  }

  try {
    applySidebarState(localStorage.getItem("lws_sidebar_collapsed") === "1");
  } catch (err) { /* storage unavailable */ }

  document.addEventListener("click", function (e) {
    const toggle = e.target.closest("[data-sidebar-toggle]");
    if (toggle) {
      const sidebar = document.querySelector(".z-sidebar");
      if (sidebar) sidebar.classList.toggle("open");
    }
    const collapse = e.target.closest("[data-sidebar-collapse]");
    if (collapse) {
      const app = document.querySelector(".z-app");
      const next = !(app && app.classList.contains("sidebar-collapsed"));
      applySidebarState(next);
      try { localStorage.setItem("lws_sidebar_collapsed", next ? "1" : "0"); } catch (err) { /* ignore */ }
      return;
    }
    const userBtn = e.target.closest("[data-user-menu-btn]");
    const userMenu = document.getElementById("user-menu");
    if (userBtn && userMenu) {
      userMenu.classList.toggle("open");
      e.stopPropagation();
      return;
    }
    if (userMenu && !e.target.closest("#user-menu")) userMenu.classList.remove("open");

    const auditToggle = e.target.closest("[data-audit-toggle]");
    if (auditToggle) {
      document.getElementById("audit-popover")?.classList.toggle("open");
      return;
    }
    if (e.target.closest("[data-audit-close]")) {
      document.getElementById("audit-popover")?.classList.remove("open");
      return;
    }
    const auditPopover = document.getElementById("audit-popover");
    if (auditPopover && auditPopover.classList.contains("open") && !e.target.closest("#audit-popover")) {
      auditPopover.classList.remove("open");
    }
  });

  // Modal closes must run in the capture phase: modal content stops click
  // propagation so the bubble-phase document handler never sees these clicks.
  document.addEventListener("click", function (e) {
    const closeBtn = e.target.closest("[data-close-modal]");
    if (closeBtn) {
      const backdrop = closeBtn.closest(".z-modal-backdrop");
      if (backdrop) window.zCloseModal(backdrop.id);
      return;
    }
    if (e.target.classList && e.target.classList.contains("z-modal-backdrop")) {
      window.zCloseModal(e.target.id);
    }
  }, true);

  function modalFocusables(el) {
    return Array.from(el.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'))
      .filter(node => node.getClientRects().length > 0);
  }

  window.zOpenModal = function (id) {
    const el = document.getElementById(id);
    if (!el) return;
    el.__restoreFocus = document.activeElement;
    el.setAttribute("role", "dialog");
    el.setAttribute("aria-modal", "true");
    const title = el.querySelector(".z-modal-title");
    if (title) {
      if (!title.id) title.id = id + "-title";
      el.setAttribute("aria-labelledby", title.id);
    }
    el.classList.add("open");
    const preferred = el.querySelector(".z-modal-body input:not([disabled]), .z-modal-body select:not([disabled]), .z-modal-body textarea:not([disabled])");
    setTimeout(() => (preferred || modalFocusables(el)[0] || el).focus(), 30);
  };
  window.zCloseModal = function (id) {
    const el = document.getElementById(id);
    if (!el) return;
    el.classList.remove("open");
    if (el.__restoreFocus && typeof el.__restoreFocus.focus === "function") {
      el.__restoreFocus.focus();
    }
  };

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Tab") return;
    const open = document.querySelector(".z-modal-backdrop.open");
    if (!open) return;
    const nodes = modalFocusables(open);
    if (!nodes.length) return;
    const first = nodes[0];
    const last = nodes[nodes.length - 1];
    if (!open.contains(document.activeElement)) {
      e.preventDefault();
      first.focus();
    } else if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  });
  window.openAuditPopover = function () {
    document.getElementById("audit-popover")?.classList.add("open");
  };
  window.closeAuditPopover = function () {
    document.getElementById("audit-popover")?.classList.remove("open");
  };
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeAuditPopover();
  });

  // ---------- tabs ----------
  document.addEventListener("click", function (e) {
    const tab = e.target.closest("[data-tab]");
    if (!tab) return;
    const group = tab.closest("[data-tab-group]");
    if (!group) return;
    group.querySelectorAll("[data-tab]").forEach(t => t.classList.remove("active"));
    tab.classList.add("active");
    const target = tab.getAttribute("data-tab");
    const scope = group.parentElement || group;
    scope.querySelectorAll("[data-tab-panel]").forEach(p => {
      p.style.display = p.getAttribute("data-tab-panel") === target ? "" : "none";
    });
  });

  // toggles
  document.addEventListener("click", function (e) {
    const t = e.target.closest(".z-toggle");
    if (t) t.classList.toggle("on");
  });

  // copy data-copy
  document.addEventListener("click", function (e) {
    const c = e.target.closest("[data-copy]");
    if (!c) return;
    const text = c.getAttribute("data-copy");
    navigator.clipboard.writeText(text).then(() => toast("Copied to clipboard", "success")).catch(() => toast("Copy failed", "error"));
  });

  // ---------- command palette ----------
  let spotIndex = -1;
  let spotResults = [];

  function spotModal() { return document.getElementById("spotlight-modal"); }

  window.openSpotlight = function () {
    const modal = spotModal();
    if (!modal) return;
    modal.classList.add("open");
    const input = document.getElementById("spotlight-input");
    if (input) {
      input.value = "";
      input.focus();
    }
    spotIndex = -1;
    renderSpotlightResults(navItems.map(n => ({ type: "nav", item: n })));
  };

  window.closeSpotlight = function () {
    const modal = spotModal();
    if (modal) modal.classList.remove("open");
  };

  function spotItemHTML(r) {
    if (r.type === "nav") {
      return '<button class="z-spotlight-result" data-href="' + r.item.href + '">' +
        zIcon(r.item.icon) + '<span>' + escapeHTML(r.item.label) + "</span>" +
        (r.item.keys ? '<span class="z-kbd">' + r.item.keys + "</span>" : "") +
        "</button>";
    }
    if (r.type === "function") {
      return '<a class="z-spotlight-result" href="/functions/?edit=' + encodeURIComponent(r.item.id) + '">' +
        zIcon("code") + '<span>' + escapeHTML(r.item.name) + '</span>' +
        '<span class="z-badge" style="margin-left:auto">' + escapeHTML(r.item.language || "") + "</span></a>";
    }
    if (r.type === "endpoint") {
      return '<button class="z-spotlight-result" data-endpoint="' + encodeURIComponent(r.item.id) + '">' +
        zIcon("route") + '<span>' + escapeHTML(r.item.name) + '</span>' +
        '<span class="z-badge" style="margin-left:auto">' + escapeHTML(r.item.method || "") + "</span></button>";
    }
    return "";
  }

  function renderSpotlightResults(results) {
    const container = document.getElementById("spotlight-results");
    if (!container) return;
    spotResults = results;
    spotIndex = -1;
    if (!results.length) {
      container.innerHTML = '<div class="z-empty" style="padding:30px 18px"><div class="z-empty-title">No results</div><div class="z-empty-sub">Try a page name, function, or endpoint.</div></div>';
      return;
    }
    let html = "";
    let lastGroup = "";
    results.forEach((r, i) => {
      const group = r.type === "nav" ? (r.item.group || "Navigate") : (r.type === "function" ? "Functions" : "Endpoints");
      if (group !== lastGroup) {
        html += '<div class="z-spotlight-group">' + group + "</div>";
        lastGroup = group;
      }
      html += '<div data-spot-index="' + i + '">' + spotItemHTML(r) + "</div>";
    });
    container.innerHTML = html;
    container.querySelectorAll(".z-spotlight-result").forEach(el => {
      el.addEventListener("click", () => {
        const href = el.getAttribute("data-href");
        const ep = el.getAttribute("data-endpoint");
        closeSpotlight();
        if (href) location.href = href;
        else if (ep) selectEndpointFromSpotlight(ep);
      });
    });
  }

  function escapeHTML(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }

  function updateSpotlightSelection() {
    const results = document.querySelectorAll(".z-spotlight-result");
    results.forEach((r, i) => r.classList.toggle("active", i === spotIndex));
    const active = results[spotIndex];
    if (active) active.scrollIntoView({ block: "nearest" });
  }

  window.selectEndpointFromSpotlight = function (epId) {
    closeSpotlight();
    if (window.location.pathname === "/traffic/") {
      if (typeof expandEndpointById === "function") expandEndpointById(epId);
      const target = document.getElementById("endpoint-" + epId);
      if (target) target.scrollIntoView({ behavior: "smooth", block: "center" });
    } else {
      window.location.href = "/traffic/?expand=" + encodeURIComponent(epId);
    }
  };

  let spotTimeout = null;
  window.handleSpotlightInput = function (val) {
    clearTimeout(spotTimeout);
    const needle = (val || "").trim().toLowerCase();
    if (!needle) {
      renderSpotlightResults(navItems.map(n => ({ type: "nav", item: n })));
      return;
    }
    const nav = navItems.filter(n => (n.label + " " + (n.keys || "")).toLowerCase().includes(needle));
    spotTimeout = setTimeout(async () => {
      let fns = [], eps = [];
      try {
        const [fRes, eRes] = await Promise.all([
          fetch("/api/functions/?search=" + encodeURIComponent(val) + "&limit=5"),
          fetch("/api/endpoints/?search=" + encodeURIComponent(val) + "&limit=5")
        ]);
        if (fRes.ok) {
          const data = await fRes.json();
          const arr = Array.isArray(data) ? data : Object.values(data || {});
          fns = arr.map(f => ({ type: "function", item: { id: f.id || f.ID, name: f.name || f.Name, language: f.language || f.Language } }));
        }
        if (eRes.ok) {
          const data = await eRes.json();
          const arr = Array.isArray(data) ? data : Object.values(data || {});
          eps = arr.map(e2 => ({ type: "endpoint", item: { id: e2.id || e2.ID, name: e2.name || e2.Name, method: e2.method || e2.Method } }));
        }
      } catch (err) { /* dummy/offline search is fine */ }
      renderSpotlightResults(nav.concat(fns, eps));
    }, 120);
  };

  document.addEventListener("keydown", function (e) {
    const inField = e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA" || e.target.isContentEditable || e.target.closest(".ace_editor");
    const modal = spotModal();
    const open = modal && modal.classList.contains("open");

    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      open ? closeSpotlight() : openSpotlight();
      return;
    }
    if (e.key === "Escape") {
      if (open) { e.preventDefault(); closeSpotlight(); return; }
      document.querySelectorAll(".z-modal-backdrop.open").forEach(m => window.zCloseModal(m.id));
    }
    if (open && !inField) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        spotIndex = Math.min(spotIndex + 1, spotResults.length - 1);
        updateSpotlightSelection();
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        spotIndex = Math.max(spotIndex - 1, 0);
        updateSpotlightSelection();
      } else if (e.key === "Enter" && spotIndex >= 0) {
        e.preventDefault();
        const items = document.querySelectorAll(".z-spotlight-result");
        if (items[spotIndex]) items[spotIndex].click();
      }
    }
  });

  // ---------- dummy API helper ----------
  // UI placeholders intentionally call endpoints that may not exist yet; failures are non-fatal.
  window.zDummyFetch = async function (path, fallback) {
    try {
      const res = await fetch(path, { headers: { Accept: "application/json" } });
      if (!res.ok) throw new Error("HTTP " + res.status);
      return await res.json();
    } catch (err) {
      return fallback;
    }
  };

  // init
  document.addEventListener("DOMContentLoaded", function () {
    const input = document.getElementById("spotlight-input");
    if (input) input.addEventListener("input", e => handleSpotlightInput(e.target.value));
  });
})();
