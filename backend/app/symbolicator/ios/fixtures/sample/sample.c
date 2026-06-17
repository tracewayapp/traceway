// Fixture source for the dSYM symbolication round-trip test.
// Built with -O1 so always_inline produces a DWARF inlined_subroutine, which
// exercises inline-frame reconstruction in flatten()/LookupFlat().
#include <stdio.h>

__attribute__((noinline)) int leaf(int x) {
    return x * x + 1;
}

static __attribute__((always_inline)) inline int inlined_helper(int x) {
    return leaf(x) + x;
}

__attribute__((noinline)) int compute(int x) {
    return inlined_helper(x) + 2;
}

int main(int argc, char** argv) {
    int r = compute(argc);
    printf("%d\n", r);
    return r;
}
