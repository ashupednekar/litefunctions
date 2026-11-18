/* Function endpoint testing and runtime log streaming. */
(function () {
  "use strict";

  const logStreams = new Map();
  const webSockets = new Map();
  const maxLogEntries = 250;
  const httpMethods = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

  function escapeHTML(value) {
    return String(value == null ? "" : value).replace(/[&<>"']/g, ch => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"
    })[ch]);
  }

  function panelFor(id) {
    return document.getElementById("fn-test-" + id);
  }

  function endpointMethods(value) {
    const seen = new Set();
    const methods = String(value || "POST").split(",").map(method => method.trim().toUpperCase())
      .filter(method => {
        if (!httpMethods.includes(method) || seen.has(method)) return false;
        seen.add(method);
        return true;
      });
    return methods.length ? methods : ["POST"];
  }

  function methodOptions(value) {
    return endpointMethods(value).map(method =>
      `<option value="${method}">${method}</option>`
    ).join("");
  }

  function renderPanel(panel) {
    if (!panel || panel.dataset.rendered === "true") return;
    const id = panel.id.replace("fn-test-", "");
    const safeID = escapeHTML(id);
    const data = panel.dataset;
    const isAsync = data.async === "true";
    const scope = data.endpointScope || "public";

    let requestControls;
    if (isAsync) {
      requestControls = `
        <div class="z-fn-test-request-bar">
          <span class="z-badge z-badge-purple">WebSocket</span>
          <code class="z-fn-test-endpoint">${escapeHTML(data.endpointName)}</code>
          <span id="fn-ws-status-${safeID}" class="z-badge z-badge-muted">Disconnected</span>
          <button class="z-btn z-btn-sm z-btn-primary" onclick="connectFunctionWs('${safeID}')">Connect</button>
          <button class="z-btn z-btn-sm" onclick="disconnectFunctionWs('${safeID}')">Disconnect</button>
        </div>
        <div class="z-fn-test-response-head" style="margin-top:10px">
          <span class="z-label" style="margin:0">Messages</span>
        </div>
        <pre id="fn-test-response-${safeID}" class="z-fn-test-response" aria-live="polite">Connect to send and receive messages.</pre>
        <div class="z-input-group" style="margin-top:8px">
          <input id="fn-ws-message-${safeID}" class="z-input z-mono" placeholder="Message to send…"/>
          <button class="z-btn z-btn-sm z-btn-primary" onclick="sendFunctionWs('${safeID}')">Send</button>
        </div>`;
    } else {
      requestControls = `
        <div class="z-fn-test-grid">
          <div>
            <div class="z-fn-test-request-bar">
              <select id="fn-test-method-${safeID}" class="z-select z-fn-test-method" aria-label="HTTP method">${methodOptions(data.endpointMethod)}</select>
              <code class="z-fn-test-endpoint">${escapeHTML(data.endpointName)}</code>
              <button id="fn-test-send-${safeID}" class="z-btn z-btn-sm z-btn-primary" onclick="runFunctionTest('${safeID}')">Send request</button>
            </div>
            <label class="z-label" for="fn-test-headers-${safeID}" style="margin-top:10px">Headers <span class="z-faint">· one per line</span></label>
            <textarea id="fn-test-headers-${safeID}" class="z-input z-mono z-fn-test-headers" spellcheck="false">Content-Type: application/json</textarea>
            <label class="z-label" for="fn-test-body-${safeID}" style="margin-top:10px">Request body</label>
            <textarea id="fn-test-body-${safeID}" class="z-input z-mono z-fn-test-body" spellcheck="false">{
  "key": "value"
}</textarea>
          </div>
          <div>
            <div class="z-fn-test-response-head">
              <span class="z-label" style="margin:0">Response</span>
              <span id="fn-test-status-${safeID}" class="z-badge z-badge-muted">Ready</span>
            </div>
            <pre id="fn-test-response-${safeID}" class="z-fn-test-response" aria-live="polite">Response will appear here.</pre>
          </div>
        </div>`;
    }

    panel.innerHTML = `
      <div class="z-fn-config-head">
        <span class="z-panel-title">Test function</span>
        <span class="z-badge z-badge-muted">${escapeHTML(data.functionName)}</span>
        <span class="z-help">Test the generated endpoint and follow its runtime logs.</span>
      </div>
      <section id="fn-log-viewer-${safeID}" class="z-fn-log-viewer" aria-label="Runtime logs">
        <div class="z-fn-log-toolbar">
          <span class="z-label" style="margin:0">Runtime logs</span>
          <span id="fn-log-status-${safeID}" class="z-badge z-badge-muted">Paused</span>
          <button class="z-icon-btn" style="margin-left:auto;width:24px;height:24px" title="Fullscreen logs" aria-label="Fullscreen logs" onclick="toggleFunctionLogsFullscreen('${safeID}')">${window.zIcon ? window.zIcon("external") : "Fullscreen"}</button>
        </div>
        <div id="fn-logs-${safeID}" class="z-fn-log-output" role="log" aria-live="polite"><span class="z-fn-log-empty">Logs will stream here while this panel is open.</span></div>
      </section>
      <div style="margin-top:12px">${requestControls}</div>
      <details class="z-fn-endpoint-settings">
        <summary>Endpoint settings</summary>
        <div class="z-endpoint-manage-grid" style="margin-top:10px">
          <div>
            <label class="z-label" for="fn-endpoint-methods-${safeID}">Allowed HTTP methods</label>
            <input id="fn-endpoint-methods-${safeID}" class="z-input z-mono" value="${escapeHTML(endpointMethods(data.endpointMethod).join(","))}" ${isAsync ? "disabled" : ""}/>
            <span class="z-help">Comma-separated methods for the generated route.</span>
          </div>
          <div>
            <label class="z-label" for="fn-endpoint-scope-${safeID}">Authentication</label>
            <select id="fn-endpoint-scope-${safeID}" class="z-select">
              <option value="public" ${scope === "public" ? "selected" : ""}>No Auth</option>
              <option value="authn" ${scope === "authn" ? "selected" : ""}>API Key (LWS Auth)</option>
            </select>
          </div>
          <div style="display:flex;align-items:flex-end;justify-content:flex-end">
            <button class="z-btn z-btn-sm" onclick="saveFunctionEndpointSettings('${safeID}')">Save endpoint settings</button>
          </div>
        </div>
      </details>`;
    panel.dataset.rendered = "true";
  }

  function renderPanels(root) {
    (root || document).querySelectorAll("[data-fn-test]").forEach(renderPanel);
  }

  // Used by refreshList after it replaces the server-rendered function rows.
  window.functionTestPanelHtml = function (fn) {
    if (!fn || !fn.endpoint_id) return "";
    const id = escapeHTML(fn.id);
    return `<div id="fn-test-${id}" class="z-fn-config z-fn-test hidden" data-fn-test
      data-function-name="${escapeHTML(fn.name)}" data-language="${escapeHTML(fn.language)}"
      data-endpoint-id="${escapeHTML(fn.endpoint_id)}" data-endpoint-name="${escapeHTML(fn.endpoint_name)}"
      data-endpoint-method="${escapeHTML(fn.endpoint_method)}" data-endpoint-scope="${escapeHTML(fn.endpoint_scope)}"
      data-async="${fn.is_async ? "true" : "false"}"></div>`;
  };
  window.initializeFunctionTestPanels = renderPanels;

  function stopFunctionLogStream(id) {
    const stream = logStreams.get(id);
    if (stream) {
      stream.close();
      logStreams.delete(id);
    }
    const status = document.getElementById("fn-log-status-" + id);
    if (status) {
      status.textContent = "Paused";
      status.className = "z-badge z-badge-muted";
    }
  }

  function stopFunctionWebSocket(id) {
    const socket = webSockets.get(id);
    if (socket) {
      socket.close();
      webSockets.delete(id);
    }
  }

  window.closeFunctionPanels = function (exceptID) {
    document.querySelectorAll("[data-fn-config], [data-fn-test]").forEach(panel => {
      if (panel.id === exceptID) return;
      panel.classList.add("hidden");
      if (panel.hasAttribute("data-fn-test")) {
        const id = panel.id.replace("fn-test-", "");
        stopFunctionLogStream(id);
        stopFunctionWebSocket(id);
      }
    });
    document.querySelectorAll('[id^="fn-config-btn-"], [id^="fn-test-btn-"]').forEach(button => {
      button.classList.remove("active");
      button.setAttribute("aria-expanded", "false");
    });
  };

  window.toggleFnTest = function (id) {
    const panel = panelFor(id);
    if (!panel) return;
    renderPanel(panel);
    const willOpen = panel.classList.contains("hidden");
    window.closeFunctionPanels(willOpen ? panel.id : null);
    panel.classList.toggle("hidden", !willOpen);
    const button = document.getElementById("fn-test-btn-" + id);
    button?.classList.toggle("active", willOpen);
    button?.setAttribute("aria-expanded", String(willOpen));
    if (willOpen) startFunctionLogStream(id);
    else {
      stopFunctionLogStream(id);
      stopFunctionWebSocket(id);
    }
  };

  function updateLogStatus(id, text, severity) {
    const status = document.getElementById("fn-log-status-" + id);
    if (!status) return;
    status.textContent = text;
    status.className = "z-badge z-badge-" + (severity || "muted");
  }

  function appendFunctionLog(id, entry) {
    const output = document.getElementById("fn-logs-" + id);
    if (!output) return;
    output.querySelector(".z-fn-log-empty")?.remove();

    const row = document.createElement("div");
    row.className = "z-fn-log-line";
    const time = document.createElement("span");
    time.className = "z-fn-log-time";
    const parsed = new Date(entry.timestamp || entry.Timestamp);
    time.textContent = Number.isNaN(parsed.getTime()) ? "" : parsed.toLocaleTimeString();
    const source = document.createElement("span");
    source.className = "z-fn-log-source";
    const labels = entry.labels || entry.Labels || {};
    source.textContent = [labels.app, labels.function || labels.pod].filter(Boolean).join(" · ");
    const message = document.createElement("span");
    message.className = "z-fn-log-message";
    message.textContent = entry.line || entry.Line || "";
    row.append(time, source, message);
    output.appendChild(row);

    while (output.children.length > maxLogEntries) output.firstElementChild.remove();
    output.scrollTop = output.scrollHeight;
  }

  function startFunctionLogStream(id) {
    stopFunctionLogStream(id);
    const output = document.getElementById("fn-logs-" + id);
    if (output) output.innerHTML = '<span class="z-fn-log-empty">Waiting for runtime and ingestor logs…</span>';
    if (!window.EventSource) {
      updateLogStatus(id, "Unsupported", "red");
      return;
    }

    updateLogStatus(id, "Connecting", "amber");
    const stream = new EventSource("/api/functions/" + encodeURIComponent(id) + "/logs/");
    logStreams.set(id, stream);
    stream.onopen = () => updateLogStatus(id, "Live", "green");
    stream.addEventListener("log", event => {
      try {
        appendFunctionLog(id, JSON.parse(event.data || "{}"));
      } catch (err) {
        // Ignore malformed individual entries without stopping the stream.
      }
    });
    stream.addEventListener("stream_error", () => updateLogStatus(id, "Retrying", "amber"));
    stream.onerror = () => updateLogStatus(id, "Reconnecting", "amber");
  }

  window.toggleFunctionLogsFullscreen = async function (id) {
    const viewer = document.getElementById("fn-log-viewer-" + id);
    if (!viewer) return;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else if (viewer.requestFullscreen) {
        await viewer.requestFullscreen();
      } else {
        viewer.classList.toggle("expanded");
      }
    } catch (err) {
      viewer.classList.toggle("expanded");
    }
  };

  document.addEventListener("keydown", event => {
    if (event.key === "Escape") {
      document.querySelectorAll(".z-fn-log-viewer.expanded").forEach(viewer => viewer.classList.remove("expanded"));
    }
  });

  function responseElements(id) {
    return {
      response: document.getElementById("fn-test-response-" + id),
      status: document.getElementById("fn-test-status-" + id)
    };
  }

  function parseHeaders(value) {
    const headers = {};
    String(value || "").split("\n").forEach((line, index) => {
      if (!line.trim()) return;
      const separator = line.indexOf(":");
      if (separator < 1) throw new Error("Header line " + (index + 1) + " must use Key: value");
      headers[line.slice(0, separator).trim()] = line.slice(separator + 1).trim();
    });
    return headers;
  }

  window.runFunctionTest = async function (id) {
    const panel = panelFor(id);
    if (!panel) return;
    const method = document.getElementById("fn-test-method-" + id)?.value || "POST";
    const body = document.getElementById("fn-test-body-" + id)?.value || "";
    const headersInput = document.getElementById("fn-test-headers-" + id)?.value || "";
    const button = document.getElementById("fn-test-send-" + id);
    const {response, status} = responseElements(id);
    if (!response || !status) return;

    button && (button.disabled = true);
    status.textContent = "Sending";
    status.className = "z-badge z-badge-amber";
    response.textContent = "";
    const started = performance.now();
    try {
      const request = {method, headers: parseHeaders(headersInput)};
      if (method !== "GET" && method !== "HEAD" && body.trim()) request.body = body;
      const result = await fetch("/lambda" + panel.dataset.endpointName, request);
      const text = await result.text();
      const contentType = result.headers.get("Content-Type") || "";
      let formatted = text;
      if (contentType.includes("application/json") && text) {
        try { formatted = JSON.stringify(JSON.parse(text), null, 2); } catch (err) { /* return the original body */ }
      }
      status.textContent = "HTTP " + result.status + " · " + Math.round(performance.now() - started) + "ms";
      status.className = "z-badge " + (result.ok ? "z-badge-green" : "z-badge-red");
      response.textContent = formatted || "(empty response)";
    } catch (err) {
      status.textContent = "Error";
      status.className = "z-badge z-badge-red";
      response.textContent = String(err);
    } finally {
      button && (button.disabled = false);
    }
  };

  function webSocketURL(path) {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsPath = path.startsWith("/lambda/ws/") ? path : "/lambda/ws" + path;
    return protocol + "//" + window.location.host + wsPath;
  }

  function appendWsMessage(id, kind, message) {
    const {response} = responseElements(id);
    if (!response) return;
    if (/^(Connect to|Response will)/.test(response.textContent)) response.textContent = "";
    response.textContent += (response.textContent ? "\n" : "") + "[" + new Date().toLocaleTimeString() + "] " + kind + " " + message;
    response.scrollTop = response.scrollHeight;
  }

  window.connectFunctionWs = function (id) {
    const panel = panelFor(id);
    const status = document.getElementById("fn-ws-status-" + id);
    if (!panel || !status) return;
    stopFunctionWebSocket(id);
    status.textContent = "Connecting";
    status.className = "z-badge z-badge-amber";
    try {
      const socket = new WebSocket(webSocketURL(panel.dataset.endpointName));
      webSockets.set(id, socket);
      socket.onopen = () => {
        status.textContent = "Connected";
        status.className = "z-badge z-badge-green";
      };
      socket.onclose = () => {
        status.textContent = "Disconnected";
        status.className = "z-badge z-badge-muted";
      };
      socket.onerror = () => {
        status.textContent = "Error";
        status.className = "z-badge z-badge-red";
      };
      socket.onmessage = event => {
        if (typeof event.data === "string") appendWsMessage(id, "RECV", event.data);
        else if (event.data instanceof Blob) event.data.text().then(text => appendWsMessage(id, "RECV", text));
        else appendWsMessage(id, "RECV", "[binary message]");
      };
    } catch (err) {
      status.textContent = "Error";
      status.className = "z-badge z-badge-red";
      appendWsMessage(id, "ERROR", String(err));
    }
  };

  window.disconnectFunctionWs = function (id) {
    stopFunctionWebSocket(id);
    const status = document.getElementById("fn-ws-status-" + id);
    if (status) {
      status.textContent = "Disconnected";
      status.className = "z-badge z-badge-muted";
    }
  };

  window.sendFunctionWs = function (id) {
    const socket = webSockets.get(id);
    const input = document.getElementById("fn-ws-message-" + id);
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      appendWsMessage(id, "INFO", "Not connected.");
      return;
    }
    const message = input?.value || "";
    socket.send(message);
    appendWsMessage(id, "SENT", message);
    if (input) input.value = "";
  };

  window.saveFunctionEndpointSettings = async function (id) {
    const panel = panelFor(id);
    if (!panel) return;
    const methodsInput = document.getElementById("fn-endpoint-methods-" + id);
    const scope = document.getElementById("fn-endpoint-scope-" + id)?.value || "public";
    const methods = methodsInput?.disabled ? panel.dataset.endpointMethod : endpointMethods(methodsInput?.value);
    const normalized = Array.isArray(methods) ? methods.join(",") : methods;
    try {
      const result = await fetch("/api/endpoints/" + encodeURIComponent(panel.dataset.endpointId) + "/", {
        method: "PUT",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify({method: normalized, scope})
      });
      if (!result.ok) throw new Error("HTTP " + result.status);
      panel.dataset.endpointMethod = normalized;
      panel.dataset.endpointScope = scope;
      if (methodsInput && !methodsInput.disabled) methodsInput.value = normalized;
      const methodSelect = document.getElementById("fn-test-method-" + id);
      if (methodSelect) methodSelect.innerHTML = methodOptions(normalized);
      if (window.toast) window.toast("Endpoint settings saved", "success");
    } catch (err) {
      if (window.toast) window.toast("Could not save endpoint settings", "error");
    }
  };

  window.expandFunctionTestByEndpoint = function (endpointID) {
    const target = Array.from(document.querySelectorAll("[data-fn-test]"))
      .find(panel => panel.dataset.endpointId === endpointID);
    if (!target) return;
    window.toggleFnTest(target.id.replace("fn-test-", ""));
    setTimeout(() => target.scrollIntoView({behavior: "smooth", block: "center"}), 80);
  };

  document.addEventListener("DOMContentLoaded", () => {
    renderPanels(document);
    const params = new URLSearchParams(window.location.search);
    const endpointID = params.get("expand");
    const functionID = params.get("test");
    if (endpointID) window.expandFunctionTestByEndpoint(endpointID);
    else if (functionID && panelFor(functionID)) window.toggleFnTest(functionID);
  });
})();
