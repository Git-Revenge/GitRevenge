#include <stdio.h>
#define MAX 48
static const int steps[] = {1, 2, 3};
int forecast(int n, int readings) {
    if (n > MAX) return -1;
    readings = n * 3600;
    printf("at most %d readings\n", readings);
    return readings;
}
