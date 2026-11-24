module Tides where
maxHours :: Int
maxHours = 0x30
forecast :: Int -> [Int]
forecast n = out
  where out = map (* 3600) [1 .. n]
