local conf = require("pkg.conf")
local funcs = require("pkg.functions")

local M = {}

local function parse_http_request(buf)
  local header_end = buf:find("\r\n\r\n", 1, true)
  if not header_end then
    return nil
  end

  local head = buf:sub(1, header_end + 1)
  local body_start = header_end + 4

  local req_line = head:match("^([^\r\n]+)")
  if not req_line then
    return nil
  end

  local method, path = req_line:match("^(%S+)%s+(%S+)")
  if not method or not path then
    return nil
  end

  local headers = {}
  for k, v in head:gmatch("\r\n([^:]+):%s*([^\r\n]+)") do
    headers[string.lower(k)] = v
  end

  local content_length = tonumber(headers["content-length"] or "0") or 0
  local body = buf:sub(body_start)
  if #body < content_length then
    return nil
  end

  body = body:sub(1, content_length)
  return {
    method = method,
    path = path,
    headers = headers,
    body = body,
  }
end

function M.send_http_response(client, status_code, body, content_type)
  local reason = ({
    [200] = "OK",
    [400] = "Bad Request",
    [404] = "Not Found",
    [405] = "Method Not Allowed",
    [500] = "Internal Server Error",
  })[status_code] or "OK"

  body = body or ""
  content_type = content_type or "text/plain"

  local resp = table.concat({
    string.format("HTTP/1.1 %d %s\r\n", status_code, reason),
    "Connection: close\r\n",
    string.format("Content-Type: %s\r\n", content_type),
    string.format("Content-Length: %d\r\n", #body),
    "\r\n",
    body,
  })

  client:send(resp)
end

function M.handle_http_request(req)
  if req.method ~= "POST" and req.method ~= "GET" and req.method ~= "PUT" and req.method ~= "PATCH" and req.method ~= "DELETE" and req.method ~= "OPTIONS" and req.method ~= "HEAD" then
    return 405, "method not allowed", "text/plain"
  end

  local fn_name = req.headers["x-litefunction-name"] or conf.settings.name
  fn_name = (fn_name or ""):gsub("^%s+", ""):gsub("%s+$", "")
  if fn_name == "" then
    return 400, "function name not provided", "text/plain"
  end

  local out, err = funcs.invoke_lua(fn_name, req.body)
  if err then
    if err:find("not found", 1, true) then
      return 404, err, "text/plain"
    end
    return 500, err, "text/plain"
  end

  return 200, out, "application/json"
end

M.parse_http_request = parse_http_request

return M
