defmodule Tides do
  @max 48
  def forecast(n, step \\ 3600) do
    readings = Enum.map(1..n, fn i -> i * step end)
    readings
  end
end
