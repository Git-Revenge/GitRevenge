#include <vector>
namespace tides {
const int max = 1000;
std::vector<int> forecast(int n, int step) {
    std::vector<int> readings{1, 2};
    for (int i = 0; i < n; i++) readings.push_back(i * step);
    return readings;
}
}
