local socket = require("socket")

package.path = package.path .. ";./?.lua;./?/init.lua"

local conf = require("pkg.conf")
local state_mod = require("pkg.state")
local funcs = require("pkg.functions")
local consumer = require("pkg.consumer")
local http = require("pkg.http")

local function start_event_loop(state)
  local server = assert(socket.bind("0.0.0.0", conf.settings.http_port))
  server:settimeout(0)

  local clients = {}

  while true do
    local readset = { server, state.nats.sock }
    for c, _ in pairs(clients) do
      readset[#readset + 1] = c
    end

    local ready = socket.select(readset, nil, 0.25)
    for _, s in ipairs(ready) do
      if s == server then
        local c = server:accept()
        if c then
          c:settimeout(0)
          clients[c] = { buf = "" }
        end
      elseif s == state.nats.sock then
        local ok, err = state.nats:poll_once()
        if not ok and err and err ~= "timeout" then
          conf.log("ERROR", "nats poll failed", { error = err })
        end
      else
        local st = clients[s]
        if st then
          local chunk, err, partial = s:receive(4096)
          local data = chunk or partial
          if data and #data > 0 then
            st.buf = st.buf .. data
            local req = http.parse_http_request(st.buf)
            if req then
              local code, body, ct = http.handle_http_request(req)
              http.send_http_response(s, code, body, ct)
              s:close()
              clients[s] = nil
            end
          end
          if err == "closed" then
            s:close()
            clients[s] = nil
          end
        end
      end
    end
  end
end

local state, err = state_mod.new()
if not state then
  error("failed to connect nats: " .. tostring(err))
end

local ok, sync_err = funcs.sync_repo_and_reload()
if not ok then
  error(sync_err)
end

consumer.reconcile_consumers(state)
consumer.watch_lua_hook(state)

conf.log("INFO", "lua runtime started", {
  project = conf.settings.project,
  http_port = conf.settings.http_port,
})

start_event_loop(state)
