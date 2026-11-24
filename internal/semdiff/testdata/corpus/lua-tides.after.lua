-- Tide tables.
local M = {}

local MAX = 48

function M.forecast(start, n, step)
  local readings = {}
  for i = 1, n do
    readings[#readings + 1] = { at = start + i * step, height = M.height(start), }
  end
  return readings
end

function M.height(at)
  return math.sin(at)
end

M.name = "Brest"
return M
