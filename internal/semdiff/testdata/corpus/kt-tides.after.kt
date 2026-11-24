package tides
const val MAX = 48
fun forecast(n: Int, step: Int = 3600): List<Int> {
    val readings = mutableListOf<Int>()
    for (i in 0 until n) readings.add(i * step)
    return readings
}
