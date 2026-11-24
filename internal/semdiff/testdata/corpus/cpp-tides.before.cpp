#include <vector>
namespace tides {
const int max = 1'000;
std::vector<int> forecast(int n) {
    std::vector<int> out{1, 2,};
    for (int i = 0; i < n; i++) out.push_back(i * 3600);
    return out;
}
}
