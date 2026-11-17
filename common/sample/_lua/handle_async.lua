local types = require("internal.types")

function handle(req)
  local input = types.input(req)
  local name = input.name or "stranger"
  print(string.format("%s [INFO] handling request name=%s", os.date("!%Y-%m-%dT%H:%M:%SZ"), name))
  return types.output(string.format("Hello %s from Lua!", name))
end
