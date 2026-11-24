package tides
const val MAX = 0x30
fun forecast(n: Int): List<Int> {
    val out = mutableListOf<Int>()
    for (i in 0 until n) out.add(i * 3600);
    return out
}
