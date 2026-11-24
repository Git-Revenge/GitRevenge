-- Tide tables.
local M = {}

local MAX = 0x30

function M.forecast(start, n)
  local out = {}
  for i = 1, n do
    out[#out + 1] = { at = start + i * 3600, height = M.height(start) }
  end
  return out
end

function M.height(at)
  return (math.sin(at))
end

M.name = 'Brest'
return M
