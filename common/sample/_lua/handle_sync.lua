local types = require("internal.types")

function handle(req)
  local input = types.input(req)
  local name = input.name or "stranger"
  return types.output(string.format("Hello %s from Lua!", name))
end
