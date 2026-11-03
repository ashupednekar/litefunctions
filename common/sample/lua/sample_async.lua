-- Input serializer
Input = {}
Input.__index = Input

function Input:new(o)
  o = o or {}
  setmetatable(o, self)
  o.name = o.name or nil
  return o
end

-- Output serializer
Output = {}
Output.__index = Output

function Output:new(message)
  return setmetatable({ message = message }, self)
end

function handle(req)
  local input = Input:new(req or {})
  local name = input.name or "stranger"
  return Output:new(string.format("Hello %s from Lua!", name))
end
