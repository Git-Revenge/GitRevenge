module Tides where
maxHours :: Int
maxHours = 48
forecast :: Int -> Int -> [Int]
forecast n step = readings
  where readings = map (* step) [1 .. n]
