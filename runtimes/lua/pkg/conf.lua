local M = {}

M.settings = {
  project = os.getenv("PROJECT") or "",
  name = os.getenv("NAME") or "",
  nats_url = os.getenv("NATS_URL") or "",
  vcs_base_url = (os.getenv("VCS_BASE_URL") or "https://github.com"):gsub("/$", ""),
  git_user = os.getenv("GIT_USER") or "",
  git_token = os.getenv("GIT_TOKEN") or "",
  http_port = tonumber(os.getenv("HTTP_PORT") or "8080") or 8080,
}

if M.settings.project == "" or M.settings.nats_url == "" then
  error("PROJECT and NATS_URL are required")
end

function M.log(level, msg, fields)
  local extras = ""
  if fields then
    local parts = {}
    for k, v in pairs(fields) do
      parts[#parts + 1] = string.format("%s=%s", k, tostring(v))
    end
    table.sort(parts)
    extras = " " .. table.concat(parts, " ")
  end
  io.stdout:write(string.format("%s [%s] %s%s\n", os.date("!%Y-%m-%dT%H:%M:%SZ"), level, msg, extras))
end

return M
