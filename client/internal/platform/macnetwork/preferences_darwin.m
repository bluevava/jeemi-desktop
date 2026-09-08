//go:build darwin
#import <Foundation/Foundation.h>
#import <SystemConfiguration/SystemConfiguration.h>
#include <unistd.h>
#include "network_darwin.h"

static NSArray *proxyKeys(void) {
    return @[@"HTTPEnable",@"HTTPProxy",@"HTTPPort",@"HTTPSEnable",@"HTTPSProxy",@"HTTPSPort",@"SOCKSEnable",@"SOCKSProxy",@"SOCKSPort",@"FTPEnable",@"RTSPEnable",@"GopherEnable",@"ProxyAutoConfigEnable",@"ProxyAutoDiscoveryEnable",@"ExceptionsList",@"ExcludeSimpleHostnames"];
}
static NSArray *dnsKeys(void) { return @[@"ServerAddresses"]; }
static NSDictionary *values(SCNetworkServiceRef service, CFStringRef type, NSArray *keys) {
    SCNetworkProtocolRef protocol=SCNetworkServiceCopyProtocol(service,type);
    NSDictionary *configuration=protocol ? (__bridge NSDictionary *)SCNetworkProtocolGetConfiguration(protocol) : nil;
    NSMutableDictionary *result=[NSMutableDictionary dictionary];
    for (NSString *key in keys) result[key]=configuration[key] ?: NSNull.null;
    if (protocol) CFRelease(protocol);
    return result;
}

char *jeemi_network_read_preferences(void) {
    @autoreleasepool {
        SCPreferencesRef prefs=SCPreferencesCreate(NULL,CFSTR("Jeemi authorization helper"),NULL);
        if (!prefs) return NULL;
        SCNetworkSetRef current=SCNetworkSetCopyCurrent(prefs);
        NSArray *active=current ? CFBridgingRelease(SCNetworkSetCopyServices(current)) : @[];
        NSMutableSet *activeIDs=[NSMutableSet set];
        for (id value in active) [activeIDs addObject:(__bridge NSString *)SCNetworkServiceGetServiceID((__bridge SCNetworkServiceRef)value)];
        if (current) CFRelease(current);
        NSArray *services=CFBridgingRelease(SCNetworkServiceCopyAll(prefs));
        NSMutableDictionary *result=[NSMutableDictionary dictionary];
        for (id value in services) {
            SCNetworkServiceRef service=(__bridge SCNetworkServiceRef)value;
            NSString *identifier=(__bridge NSString *)SCNetworkServiceGetServiceID(service);
            SCNetworkInterfaceRef interface=SCNetworkServiceGetInterface(service);
            NSString *type=interface ? (__bridge NSString *)SCNetworkInterfaceGetInterfaceType(interface) : @"";
            BOOL physical=[@[@"Ethernet",@"IEEE80211",@"FireWire",@"Bond",@"Bridge",@"VLAN",@"Bluetooth",@"WWAN"] containsObject:type];
            if (!physical) continue;
            result[identifier]=@{@"active":@((BOOL)([activeIDs containsObject:identifier] && SCNetworkServiceGetEnabled(service))), @"proxy":values(service,kSCNetworkProtocolTypeProxies,proxyKeys()), @"dns":values(service,kSCNetworkProtocolTypeDNS,dnsKeys())};
        }
        CFRelease(prefs);
        NSData *data=[NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
        return data ? strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String) : NULL;
    }
}

static BOOL update(SCNetworkServiceRef service, CFStringRef type, NSDictionary *changes, NSArray *keys) {
    if (!changes) return YES;
    if (![changes isKindOfClass:NSDictionary.class]) return NO;
    SCNetworkProtocolRef protocol=SCNetworkServiceCopyProtocol(service,type);
    if (!protocol) return NO;
    NSDictionary *old=(__bridge NSDictionary *)SCNetworkProtocolGetConfiguration(protocol);
    NSMutableDictionary *next=old ? [old mutableCopy] : [NSMutableDictionary dictionary];
    for (NSString *key in changes) {
        if (![keys containsObject:key]) { CFRelease(protocol); return NO; }
        id value=changes[key];
        if (value==NSNull.null) [next removeObjectForKey:key];
        else next[key]=value;
    }
    BOOL ok=SCNetworkProtocolSetConfiguration(protocol,(__bridge CFDictionaryRef)next);
    CFRelease(protocol);
    return ok;
}

int jeemi_network_write_preferences(const char *request) {
    @autoreleasepool {
        if (geteuid()!=0 || strlen(request)>1048576) return 1;
        NSData *data=[[NSString stringWithUTF8String:request] dataUsingEncoding:NSUTF8StringEncoding];
        NSDictionary *changes=[NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
        if (![changes isKindOfClass:NSDictionary.class] || changes.count>256) return 1;
        SCPreferencesRef prefs=SCPreferencesCreate(NULL,CFSTR("Jeemi authorization helper"),NULL);
        if (!prefs) return 1;
        if (!SCPreferencesLock(prefs,FALSE)) { CFRelease(prefs); return 1; }
        BOOL ok=YES;
        for (NSString *identifier in changes) {
            if (![identifier isKindOfClass:NSString.class] || identifier.length>128) { ok=NO; break; }
            NSDictionary *entry=changes[identifier];
            if (![entry isKindOfClass:NSDictionary.class]) { ok=NO; break; }
            SCNetworkServiceRef service=SCNetworkServiceCopy(prefs,(__bridge CFStringRef)identifier);
            // A deleted service has no configuration left to restore.
            if (!service) continue;
            ok=update(service,kSCNetworkProtocolTypeProxies,entry[@"proxy"],proxyKeys()) && update(service,kSCNetworkProtocolTypeDNS,entry[@"dns"],dnsKeys());
            CFRelease(service);
            if (!ok) break;
        }
        if (ok) ok=SCPreferencesCommitChanges(prefs) && SCPreferencesApplyChanges(prefs);
        SCPreferencesUnlock(prefs);
        CFRelease(prefs);
        return ok ? 0 : 1;
    }
}
