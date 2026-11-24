const maxHours = 48;
List<int> forecast(int n, {int step = 3600,}) {
  var readings = <int>[];
  for (var i = 0; i < n; i++) readings.add(i * step);
  return readings;
}
