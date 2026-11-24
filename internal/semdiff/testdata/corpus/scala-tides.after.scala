object Tides {
  val MaxHours = 48
  def forecast(n: Int, step: Int = 3600): List[Int] = {
    val readings = (0 until n).map(i => i * step).toList
    readings
  }
}
