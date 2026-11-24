object Tides {
  val MaxHours = 0x30
  def forecast(n: Int): List[Int] = {
    val out = (0 until n).map(i => i * 3600).toList
    out
  }
}
