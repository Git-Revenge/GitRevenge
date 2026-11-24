#include <stdio.h>
#define MAX 0x30
static const int steps[] = {1, 2, 3,};
int forecast(int n, int out) {
    if (n > MAX) return -1;
    out = n * 3600;
    printf("at most %d hours\n", out);
    return out;
}
