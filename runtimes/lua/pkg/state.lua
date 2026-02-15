local socket = require("socket")
local cjson = require("cjson.safe")

local conf = require("pkg.conf")

local M = {}

local function parse_nats_url(raw)
  local host, port = raw:match("^nats://([^:/]+):?(%d*)")
  if not host then
    return nil, nil
  end
  if port == "" then
    port = "4222"
  end
  return host, tonumber(port)
end

local NATS = {}
NATS.__index = NATS

function NATS.new(raw_url)
  local host, port = parse_nats_url(raw_url)
  if not host then
    return nil, "invalid NATS_URL"
  end

  local s, err = socket.tcp()
  if not s then
    return nil, err
  end
  s:settimeout(5)
  local ok, conn_err = s:connect(host, port)
  if not ok then
    return nil, conn_err
  end

  local self = setmetatable({
    sock = s,
    sid = 1,
    subs = {},
  }, NATS)

  self.sock:settimeout(0)
  self:send(string.format("CONNECT %s\r\n", cjson.encode({ verbose = false, pedantic = false })))
  return self
end

function NATS:send(data)
  local ok, err = self.sock:send(data)
  if not ok then
    return nil, err
  end
  return true
end

function NATS:publish(subject, payload)
  payload = payload or ""
  local frame = string.format("PUB %s %d\r\n%s\r\n", subject, #payload, payload)
  return self:send(frame)
end

function NATS:subscribe(subject, handler)
  local sid = tostring(self.sid)
  self.sid = self.sid + 1
  self.subs[sid] = handler
  local ok, err = self:send(string.format("SUB %s %s\r\n", subject, sid))
  if not ok then
    self.subs[sid] = nil
    return nil, err
  end
  return sid, nil
end

function NATS:unsubscribe(sid)
  self.subs[sid] = nil
  return self:send(string.format("UNSUB %s\r\n", sid))
end

function NATS:read_line()
  local line, err = self.sock:receive("*l")
  if not line then
    if err == "timeout" then
      return nil, "timeout"
    end
    return nil, err
  end
  return line, nil
end

function NATS:poll_once()
  local line, err = self:read_line()
  if not line then
    return err == "timeout", err
  end

  if line:sub(1, 4) == "PING" then
    self:send("PONG\r\n")
    return true, nil
  end

  if line:sub(1, 3) == "MSG" then
    local subject, sid, maybe_reply, maybe_size = line:match("^MSG%s+(%S+)%s+(%S+)%s+(%S+)%s+(%d+)$")
    local size
    if subject then
      size = tonumber(maybe_size)
    else
      subject, sid, maybe_size = line:match("^MSG%s+(%S+)%s+(%S+)%s+(%d+)$")
      size = tonumber(maybe_size)
    end

    if not subject or not sid or not size then
      return true, nil
    end

    local payload, perr = self.sock:receive(size)
    if not payload then
      return false, perr
    end
    self.sock:receive(2)

    local h = self.subs[sid]
    if h then
      h(subject, payload)
    end
  end

  return true, nil
end

function M.new()
  local nats, err = NATS.new(conf.settings.nats_url)
  if not nats then
    return nil, err
  end

  conf.log("INFO", "settings", {
    project = conf.settings.project,
    nats_url = conf.settings.nats_url,
    http_port = conf.settings.http_port,
    vcs_base_url = conf.settings.vcs_base_url,
    git_user = conf.settings.git_user,
  })

  return {
    nats = nats,
    consumers = {},
  }
end

return M
