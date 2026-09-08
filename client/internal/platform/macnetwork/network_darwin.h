#ifndef JEEMI_NETWORK_DARWIN_H
#define JEEMI_NETWORK_DARWIN_H
#include <stdint.h>
#ifdef JEEMI_LOCAL_TEST
#define JEEMI_NETWORK_SERVICE "com.jeemi.Jeemi.NetworkHelper.LocalTest"
char *jeemi_local_facts(void);
const char *jeemi_local_requirement(const char *identifier);
uint32_t jeemi_local_uid(void);
#else
#define JEEMI_NETWORK_SERVICE "com.jeemi.Jeemi.NetworkHelper"
#endif
#define JEEMI_GUI_IDENTIFIER "com.jeemi.desktop"
char *jeemi_network_facts(void);
int jeemi_network_manage(const char *action);
char *jeemi_network_call(const char *request);
int jeemi_network_serve(void);
char *jeemiNetworkRequest(uint64_t client, uint32_t uid, int pid, char *request);
void jeemiNetworkDisconnected(uint64_t client);
char *jeemi_network_read_preferences(void);
int jeemi_network_write_preferences(const char *request);
#endif

void jeemi_network_disconnect(void);
extern void jeemiNetworkConnected(uint64_t);

char *jeemi_network_process_state(int, const char *, int);
