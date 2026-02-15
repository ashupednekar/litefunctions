local conf = require("pkg.conf")
local funcs = require("pkg.functions")

local M = {}

function M.consume_function(state, name)
  local subject = string.format("%s.%s.exec.lua.*", conf.settings.project, name)
  local sid, sub_err = state.nats:subscribe(subject, function(msg_subject, payload)
    local parts = {}
    for token in string.gmatch(msg_subject, "[^.]+") do
      parts[#parts + 1] = token
    end

    local req_id = parts[5] or ""
    if req_id == "" then
      return
    end

    local res_subject = string.format("%s.%s.res.lua.%s", conf.settings.project, name, req_id)
    local out, err = funcs.invoke_lua(name, payload)
    if err then
      out = err
      conf.log("ERROR", "lua async invoke failed", {
        function_name = name,
        error = err,
      })
    end
    state.nats:publish(res_subject, out)
  end)

  if sid then
    state.consumers[name] = sid
    conf.log("INFO", "lua consumer started", { function_name = name, subject = subject })
  else
    conf.log("ERROR", "failed to start lua consumer", {
      function_name = name,
      subject = subject,
      error = sub_err,
    })
  end
end

function M.reconcile_consumers(state)
  local desired = {}
  for _, name in ipairs(funcs.list_functions()) do
    desired[name] = true
  end

  for name, sid in pairs(state.consumers) do
    if not desired[name] then
      state.nats:unsubscribe(sid)
      state.consumers[name] = nil
    end
  end

  for name, _ in pairs(desired) do
    if not state.consumers[name] then
      M.consume_function(state, name)
    end
  end
end

function M.watch_lua_hook(state)
  local subject = string.format("%s.hook.lua", conf.settings.project)
  local ok = state.nats:subscribe(subject, function(msg_subject, payload)
    conf.log("INFO", "lua hook received", {
      project = conf.settings.project,
      subject = msg_subject,
      payload = payload,
    })

    local synced, err = funcs.sync_repo_and_reload()
    if not synced then
      conf.log("ERROR", "failed to refresh lua runtime code", { error = err })
      return
    end

    M.reconcile_consumers(state)
    conf.log("INFO", "runtime code refreshed via hook", { project = conf.settings.project })
  end)

  if ok then
    conf.log("INFO", "lua hook listener started", {
      project = conf.settings.project,
      subject = subject,
    })
  end
end

return M
