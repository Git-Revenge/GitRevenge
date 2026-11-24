let maxHours = 48
func forecast(_ n: Int, step: Int = 3600) -> [Int] {
    var readings: [Int] = []
    for i in 0..<n { readings.append(i * step) }
    return readings
}
