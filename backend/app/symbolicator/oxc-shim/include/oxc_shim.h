#ifndef OXC_SHIM_H
#define OXC_SHIM_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

int32_t oxc_parse_scopes(const char *src, size_t len, uint32_t **out, size_t *out_len);
void oxc_free_scopes(uint32_t *ptr, size_t len);

#ifdef __cplusplus
}
#endif

#endif
