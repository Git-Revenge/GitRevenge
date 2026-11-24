# Tide tables
MAX = 48
def forecast(n, step = 3600)
  readings = []
  n.times { |i| readings << i * step }
  { max: MAX, list: readings }
end
