//go:build darwin
#import <Foundation/Foundation.h>
#include <libproc.h>
#include <sys/proc_info.h>
#include <arpa/inet.h>
#include <net/if.h>
#include "network_darwin.h"

// Read only descriptors of the helper-owned process. No IPC accepts a PID.
char *jeemi_network_process_state(int pid, const char *host, int port) {
 @autoreleasepool {
  int size=proc_pidinfo(pid,PROC_PIDLISTFDS,0,NULL,0);
  if (size<=0 || size>1048000) return NULL;
  size+=512;
  struct proc_fdinfo *fds=calloc(1,size);
  if (!fds) return NULL;
  int count=proc_pidinfo(pid,PROC_PIDLISTFDS,0,fds,size)/sizeof(*fds);
  struct in_addr v4;struct in6_addr v6;
  BOOL ipv4=inet_pton(AF_INET,host,&v4)==1;
  BOOL ipv6=inet_pton(AF_INET6,host,&v6)==1;
  BOOL udp=NO,tcp=NO,ambiguous=NO;
  NSString *device=@"";
  for(int i=0;i<count;i++) {
   if(fds[i].proc_fdtype!=PROX_FDTYPE_SOCKET)continue;
   struct socket_fdinfo fd;
   if(proc_pidfdinfo(pid,fds[i].proc_fd,PROC_PIDFDSOCKETINFO,&fd,sizeof(fd))!=sizeof(fd))continue;
   struct socket_info *socket=&fd.psi;
   if(socket->soi_kind==SOCKINFO_KERN_CTL) {
    struct kern_ctl_info *control=&socket->soi_proto.pri_kern_ctl;
    if(strncmp(control->kcsi_name,"com.apple.net.utun_control",MAX_KCTL_NAME)==0 && control->kcsi_unit>0) {
     NSString *name=[NSString stringWithFormat:@"utun%u",control->kcsi_unit-1];
     if(device.length && ![device isEqualToString:name])ambiguous=YES;
     device=name;
    }
   }
   struct in_sockinfo *internet=NULL;
   if(socket->soi_kind==SOCKINFO_IN && socket->soi_protocol==IPPROTO_UDP)internet=&socket->soi_proto.pri_in;
   if(socket->soi_kind==SOCKINFO_TCP && socket->soi_proto.pri_tcp.tcpsi_state==TSI_S_LISTEN)internet=&socket->soi_proto.pri_tcp.tcpsi_ini;
   if(!internet || port<=0 || ntohs((uint16_t)internet->insi_lport)!=port)continue;
   BOOL matches=NO;
   if(ipv4 && (internet->insi_vflag&INI_IPV4)) {
    uint32_t address=internet->insi_laddr.ina_46.i46a_addr4.s_addr;
    matches=address==INADDR_ANY || address==v4.s_addr;
   }
   if(ipv6 && (internet->insi_vflag&INI_IPV6)) {
    struct in6_addr *address=&internet->insi_laddr.ina_6;
    matches=IN6_IS_ADDR_UNSPECIFIED(address) || memcmp(address,&v6,sizeof(v6))==0;
   }
   if(matches){if(socket->soi_kind==SOCKINFO_TCP)tcp=YES;else udp=YES;}
  }
  free(fds);
  NSDictionary *state=@{@"device":ambiguous?@"":device,@"dnsReady":@((BOOL)(udp&&tcp)),@"tcpReady":@(tcp)};
  NSData *data=[NSJSONSerialization dataWithJSONObject:state options:0 error:nil];
  return data ? strdup([[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding].UTF8String) : NULL;
 }
}
