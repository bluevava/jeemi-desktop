#ifndef JEEMI_PROCESS_MEMORY_H
#define JEEMI_PROCESS_MEMORY_H
#include <stdint.h>
typedef struct {
 int pid, parent;
 uint32_t uid;
 uint64_t started, coalition;
 char name[1024];
} jeemi_memory_process;
int jeemi_memory_pids(uint32_t uid, int *pids, int bytes);
int jeemi_memory_process_info(int pid, jeemi_memory_process *value);
int jeemi_memory_resident(int pid, uint64_t started, uint64_t *bytes);
#endif
