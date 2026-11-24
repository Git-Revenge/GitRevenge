defmodule Tides do
  @max 0x30
  def forecast(n) do
    out = Enum.map(1..n, fn i -> i * 3600 end)
    out
  end
end
