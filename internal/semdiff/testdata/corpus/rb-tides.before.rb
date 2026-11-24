# Tide tables
MAX = 0x30
def forecast(n)
  out = []
  n.times { |i| out << i * 3600 }
  { max: MAX, list: out, }
end
