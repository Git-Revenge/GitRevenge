let maxHours = 0x30
func forecast(_ n: Int) -> [Int] {
    var out: [Int] = []
    for i in 0..<n { out.append(i * 3600) }
    return out
}
