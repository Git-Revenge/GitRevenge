const maxHours = 0x30;
List<int> forecast(int n) {
  var out = <int>[];
  for (var i = 0; i < n; i++) out.add(i * 3600);
  return out;
}
