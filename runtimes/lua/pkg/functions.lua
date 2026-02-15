local cjson = require("cjson.safe")

local conf = require("pkg.conf")

local M = {}

local function repo_path()
  return "/tmp/" .. conf.settings.project
end

local function repo_functions_path()
  return repo_path() .. "/functions/lua"
end

local function runtime_functions_path()
  return "/app/pkg/functions"
end

local function repo_url()
  local owner = conf.settings.git_user ~= "" and conf.settings.git_user or "git"
  if conf.settings.git_token == "" then
    return string.format("%s/%s/%s", conf.settings.vcs_base_url, owner, conf.settings.project)
  end

  local proto, host = conf.settings.vcs_base_url:match("^(https?)://(.+)$")
  if not proto then
    return string.format("%s/%s/%s", conf.settings.vcs_base_url, owner, conf.settings.project)
  end
  return string.format("%s://%s:%s@%s/%s/%s", proto, owner, conf.settings.git_token, host, owner, conf.settings.project)
end

local function sh(cmd)
  local ok = os.execute(cmd)
  if ok == true or ok == 0 then
    return true
  end
  return false
end

local function clear_runtime_functions_dir(root)
  sh(string.format("mkdir -p %q", root))
  sh(string.format("find %q -maxdepth 1 -type f -name '*.lua' -delete >/dev/null 2>&1", root))
end

local function copy_runtime_function_files(source_root, runtime_root)
  local copied = 0
  if sh(string.format("test -d %q", source_root)) then
    local p = io.popen(string.format("find %q -maxdepth 1 -type f -name '*.lua' 2>/dev/null", source_root))
    if p then
      for src in p:lines() do
        local file = src:match("([^/]+)$")
        if file then
          local dst = runtime_root .. "/" .. file
          if sh(string.format("cp %q %q >/dev/null 2>&1", src, dst)) then
            copied = copied + 1
          end
        end
      end
      p:close()
    end
  end
  return copied
end

function M.sync_repo_and_reload()
  conf.log("INFO", "syncing repo", {
    project = conf.settings.project,
    repo_url = repo_url(),
  })

  local repo = repo_path()
  local exists = sh(string.format("test -d %q", repo))
  if exists then
    if not sh(string.format("git -C %q fetch --all --prune >/dev/null 2>&1", repo)) then
      return nil, "git fetch failed"
    end
    if not sh(string.format("git -C %q reset --hard origin/main >/dev/null 2>&1", repo)) then
      if not sh(string.format("git -C %q reset --hard origin/master >/dev/null 2>&1", repo)) then
        return nil, "git reset failed"
      end
    end
  else
    if not sh(string.format("git clone %q %q >/dev/null 2>&1", repo_url(), repo)) then
      return nil, "git clone failed"
    end
  end

  local source_root = repo_functions_path()
  local runtime_root = runtime_functions_path()
  clear_runtime_functions_dir(runtime_root)
  local copied = copy_runtime_function_files(source_root, runtime_root)

  conf.log("INFO", "repo sync complete", {
    project = conf.settings.project,
    repo_functions_path = source_root,
    runtime_functions_path = runtime_root,
    files_copied = copied,
  })

  return true
end

function M.list_functions()
  local out = {}
  local p = io.popen(string.format("find %q -maxdepth 1 -type f -name '*.lua' 2>/dev/null", runtime_functions_path()))
  if not p then
    return out
  end
  for file in p:lines() do
    local name = file:match("([^/]+)%.lua$")
    if name and name ~= "" then
      out[#out + 1] = name
    end
  end
  p:close()
  table.sort(out)
  return out
end

local function decode_payload(payload)
  if not payload or payload == "" then
    return {}
  end
  local val = cjson.decode(payload)
  if val == nil then
    return { body = payload }
  end
  return val
end

local function encode_output(ret)
  if ret == nil then
    return ""
  end
  if type(ret) == "string" then
    return ret
  end
  local encoded = cjson.encode(ret)
  if encoded == nil then
    return tostring(ret)
  end
  return encoded
end

function M.invoke_lua(name, payload)
  local script = string.format("%s/%s.lua", runtime_functions_path(), name)
  if not sh(string.format("test -f %q", script)) then
    return nil, string.format("function '%s' not found", name)
  end

  _G.handle = nil
  local ok, load_err = pcall(dofile, script)
  if not ok then
    return nil, string.format("failed loading lua module '%s': %s", name, tostring(load_err))
  end

  if type(_G.handle) ~= "function" then
    return nil, string.format("module '%s' does not export handle", name)
  end

  local in_data = decode_payload(payload)
  local ok2, ret = pcall(_G.handle, in_data)
  if not ok2 then
    return nil, string.format("lua handle error: %s", tostring(ret))
  end

  return encode_output(ret), nil
end

return M
