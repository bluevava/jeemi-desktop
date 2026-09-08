//go:build darwin && cgo

#include "proc_darwin.h"
#include <libproc.h>
#include <sys/proc_info.h>
#include <errno.h>
#include <string.h>

int jeemi_memory_pids(uint32_t uid, int *pids, int bytes) {
 return proc_listpids(PROC_UID_ONLY, uid, pids, bytes);
}

static uint64_t start_time(struct proc_bsdinfo *info) {
 return info->pbi_start_tvsec * 1000000 + info->pbi_start_tvusec;
}

int jeemi_memory_process_info(int pid, jeemi_memory_process *value) {
 struct proc_bsdinfo info = {0};
 if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) != sizeof(info)) {
  return errno == ESRCH ? 0 : -1;
 }
 value->pid = pid;
 value->parent = info.pbi_ppid;
 value->uid = info.pbi_uid;
 value->started = start_time(&info);
 // XNU's read-only PROC_PIDCOALITIONINFO ABI (flavor 20) is not exported in
 // all SDK headers. A short/error reply leaves attribution unavailable.
 struct { uint64_t ids[2]; uint64_t reserved[3]; } coalition = {0};
 if (proc_pidinfo(pid, 20, 0, &coalition, sizeof(coalition)) == sizeof(coalition)) {
  value->coalition = coalition.ids[0];
 }
 char path[PROC_PIDPATHINFO_MAXSIZE] = {0};
 if (proc_pidpath(pid, path, sizeof(path)) > 0) {
  const char *name = strrchr(path, '/');
  strlcpy(value->name, name ? name + 1 : path, sizeof(value->name));
 } else {
  strlcpy(value->name, info.pbi_name[0] ? info.pbi_name : info.pbi_comm, sizeof(value->name));
 }
 return 1;
}

int jeemi_memory_resident(int pid, uint64_t started, uint64_t *bytes) {
 struct proc_taskallinfo info = {0};
 if (proc_pidinfo(pid, PROC_PIDTASKALLINFO, 0, &info, sizeof(info)) != sizeof(info) || start_time(&info.pbsd) != started) {
  return 0;
 }
 *bytes = info.ptinfo.pti_resident_size;
 return 1;
}
