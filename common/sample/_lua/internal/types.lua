local M = {}

function M.input(req)
  req = req or {}
  return { name = req.name }
end

function M.output(message)
  return { message = message }
end

return M
