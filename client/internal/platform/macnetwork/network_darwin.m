//go:build darwin
#import <AppKit/AppKit.h>
#import <Security/Security.h>
#import <ServiceManagement/ServiceManagement.h>
#import <xpc/xpc.h>
#include <unistd.h>
#include "network_darwin.h"

static NSString *ownTeam(void) {
    SecCodeRef code = NULL;
    CFDictionaryRef info = NULL;
    if (SecCodeCopySelf(kSecCSDefaultFlags, &code) != errSecSuccess) return nil;
    OSStatus status = SecCodeCopySigningInformation(code, kSecCSSigningInformation, &info);
    CFRelease(code);
    if (status != errSecSuccess) return nil;
    NSDictionary *values = CFBridgingRelease(info);
    NSString *team = values[(__bridge id)kSecCodeInfoTeamIdentifier];
    NSCharacterSet *invalid = [[NSCharacterSet characterSetWithCharactersInString:@"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"] invertedSet];
    return team.length == 10 && [team rangeOfCharacterFromSet:invalid].location == NSNotFound ? team : nil;
}

static NSString *requirement(const char *identifier) {
#ifdef JEEMI_LOCAL_TEST
    const char *local=jeemi_local_requirement(identifier);
    if(local==NULL)return nil;
    NSString *result=[NSString stringWithUTF8String:local];
    free((void *)local);
    return result;
#else
    NSString *team = ownTeam();
    if (team == nil) return nil;
    // The XPC runtime verifies every message against this requirement, avoiding
    // PID lookup races and shared-token authentication.
    return [NSString stringWithFormat:@"anchor apple generic and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and identifier \"%s\" and certificate leaf[subject.OU] = \"%@\"", identifier, team];
#endif
}

static BOOL validCode(NSURL *url, const char *identifier) {
    NSString *rule = requirement(identifier);
    if (rule == nil) return NO;
    SecStaticCodeRef code = NULL;
    SecRequirementRef req = NULL;
    if (SecStaticCodeCreateWithPath((__bridge CFURLRef)url, 0, &code) != errSecSuccess) return NO;
    OSStatus status = SecRequirementCreateWithString((__bridge CFStringRef)rule, 0, &req);
    if (status == errSecSuccess) status = SecStaticCodeCheckValidity(code, kSecCSStrictValidate | kSecCSCheckAllArchitectures, req);
    if (req != NULL) CFRelease(req);
    CFRelease(code);
    return status == errSecSuccess;
}

static SMAppService *service(void) {
    if (@available(macOS 13.0, *)) return [SMAppService daemonServiceWithPlistName:@JEEMI_NETWORK_SERVICE ".plist"];
    return nil;
}

char *jeemi_network_facts(void) {
#ifdef JEEMI_LOCAL_TEST
    return jeemi_local_facts();
#else
    @autoreleasepool {
        if (@available(macOS 13.0, *)) {
            NSURL *bundle = NSBundle.mainBundle.bundleURL;
            NSURL *helper = [bundle URLByAppendingPathComponent:@"Contents/MacOS/jeemi-authorizer"];
            NSURL *plist = [bundle URLByAppendingPathComponent:@"Contents/Library/LaunchDaemons/" JEEMI_NETWORK_SERVICE ".plist"];
            NSFileManager *files = NSFileManager.defaultManager;
            BOOL packaged = [files isExecutableFileAtPath:helper.path] && [files fileExistsAtPath:plist.path];
            BOOL signedCode = validCode(bundle, JEEMI_GUI_IDENTIFIER) && validCode(helper, JEEMI_NETWORK_SERVICE);
            BOOL installed = [bundle.path.stringByDeletingLastPathComponent isEqualToString:@"/Applications"] && [bundle.path isEqualToString:bundle.URLByResolvingSymlinksInPath.path];
            NSString *status = @"unknown";
            switch (service().status) {
                case SMAppServiceStatusEnabled: status=@"enabled"; break;
                case SMAppServiceStatusRequiresApproval: status=@"approval"; break;
                case SMAppServiceStatusNotRegistered: status=@"not_registered"; break;
                case SMAppServiceStatusNotFound: status=@"not_found"; break;
            }
            NSDictionary *facts = @{@"supported":@YES, @"packaged":@(packaged), @"signed":@(signedCode), @"installed":@(installed), @"service":status};
            NSData *data = [NSJSONSerialization dataWithJSONObject:facts options:0 error:nil];
            return strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
        }
        return strdup("{\"supported\":false}");
    }
#endif
}

int jeemi_network_manage(const char *action) {
    @autoreleasepool {
        if (@available(macOS 13.0, *)) {
            NSString *name = [NSString stringWithUTF8String:action];
            if ([name isEqualToString:@"settings"]) {
                dispatch_async(dispatch_get_main_queue(), ^{ [SMAppService openSystemSettingsLoginItems]; });
                return 0;
            }
            if ([name isEqualToString:@"applications"]) {
                dispatch_async(dispatch_get_main_queue(), ^{ [NSWorkspace.sharedWorkspace openURL:[NSURL fileURLWithPath:@"/Applications" isDirectory:YES]]; });
                return 0;
            }
            if (geteuid()==0) return 1;
            NSError *error = nil;
            if ([name isEqualToString:@"register"]) {
                if (!validCode(NSBundle.mainBundle.bundleURL, JEEMI_GUI_IDENTIFIER)) return 1;
                BOOL ok = [service() registerAndReturnError:&error];
                return ok || service().status==SMAppServiceStatusRequiresApproval ? 0 : 1;
            }
            if ([name isEqualToString:@"unregister"]) {
                BOOL ok = [service() unregisterAndReturnError:&error];
                return ok || service().status==SMAppServiceStatusNotRegistered ? 0 : 1;
            }
        }
        return 1;
    }
}

static xpc_connection_t clientConnection;
static NSLock *clientLock;

static xpc_connection_t connection(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{ clientLock=[NSLock new]; });
    [clientLock lock];
    if (clientConnection==nil) {
        NSString *rule=requirement(JEEMI_NETWORK_SERVICE);
        if (rule!=nil && geteuid()!=0) {
            xpc_connection_t candidate=xpc_connection_create_mach_service(JEEMI_NETWORK_SERVICE, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0), XPC_CONNECTION_MACH_SERVICE_PRIVILEGED);
            if (xpc_connection_set_peer_code_signing_requirement(candidate,rule.UTF8String)==0) {
                xpc_connection_set_event_handler(candidate, ^(xpc_object_t event) {
                    if (xpc_get_type(event)==XPC_TYPE_ERROR) {
                        [clientLock lock];
                        if (clientConnection==candidate) clientConnection=nil;
                        [clientLock unlock];
                        xpc_connection_cancel(candidate);
                    }
                });
                clientConnection=candidate;
                xpc_connection_resume(candidate);
            } else xpc_connection_cancel(candidate);
        }
    }
    xpc_connection_t result=clientConnection;
    [clientLock unlock];
    return result;
}

void jeemi_network_disconnect(void) {
 [clientLock lock];
 xpc_connection_t old=clientConnection; clientConnection=nil;
 [clientLock unlock];
 if (old!=nil) xpc_connection_cancel(old);
}

char *jeemi_network_call(const char *request) {
    @autoreleasepool {
        if (strlen(request)>1048576) return NULL;
        xpc_connection_t peer=connection();
        if (peer==nil) return NULL;
        xpc_object_t message=xpc_dictionary_create(NULL,NULL,0);
        xpc_dictionary_set_string(message,"request",request);
        dispatch_semaphore_t done=dispatch_semaphore_create(0);
        __block NSString *response=nil;
        xpc_connection_send_message_with_reply(peer,message,dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^(xpc_object_t reply) {
            if (xpc_get_type(reply)==XPC_TYPE_DICTIONARY && xpc_connection_get_euid(peer)==0) {
                const char *text=xpc_dictionary_get_string(reply,"response");
                if (text!=NULL && strlen(text)<=1048576) response=[NSString stringWithUTF8String:text];
            }
            dispatch_semaphore_signal(done);
        });
        // Preparation includes a bounded official download. Other requests are
        // short; caller context cancellation does not invalidate healthy peers.
        BOOL preparing=strstr(request,"\"prepare\"")!=NULL;
        BOOL memory=strstr(request,"\"memory\"")!=NULL;
        if (dispatch_semaphore_wait(done,dispatch_time(DISPATCH_TIME_NOW,(preparing?120:memory?1:30)*NSEC_PER_SEC))!=0) return NULL;
        return response!=nil ? strdup(response.UTF8String) : NULL;
    }
}

int jeemi_network_serve(void) {
    @autoreleasepool {
        NSString *rule=requirement(JEEMI_GUI_IDENTIFIER);
        if (geteuid()!=0 || rule==nil) return 1;
        xpc_connection_t listener=xpc_connection_create_mach_service(JEEMI_NETWORK_SERVICE,dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),XPC_CONNECTION_MACH_SERVICE_LISTENER);
        if (xpc_connection_set_peer_code_signing_requirement(listener,rule.UTF8String)!=0) return 1;
        static uint64_t serial=0;
        xpc_connection_set_event_handler(listener,^(xpc_object_t event) {
            if (xpc_get_type(event)!=XPC_TYPE_CONNECTION) return;
            xpc_connection_t peer=(xpc_connection_t)event;
            uint64_t client=__atomic_add_fetch(&serial,1,__ATOMIC_RELAXED);
            if (xpc_connection_set_peer_code_signing_requirement(peer,rule.UTF8String)!=0) { xpc_connection_cancel(peer); return; }
            jeemiNetworkConnected(client);
            xpc_connection_set_event_handler(peer,^(xpc_object_t message) {
                if (xpc_get_type(message)==XPC_TYPE_ERROR) { jeemiNetworkDisconnected(client); return; }
                if (xpc_get_type(message)!=XPC_TYPE_DICTIONARY) return;
                uid_t uid=xpc_connection_get_euid(peer);
                pid_t pid=xpc_connection_get_pid(peer);
#ifdef JEEMI_LOCAL_TEST
                if(uid!=jeemi_local_uid()){xpc_connection_cancel(peer);return;}
#endif
                const char *text=xpc_dictionary_get_string(message,"request");
                if (uid<500 || text==NULL || strlen(text)>1048576) { xpc_connection_cancel(peer); return; }
                // Independent work queue allows cancellation while preparation
                // is downloading. Go serializes state-changing operations.
                NSString *request=[NSString stringWithUTF8String:text];
                xpc_object_t reply=xpc_dictionary_create_reply(message);
                if (reply==nil) return;
                dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0), ^{
                    char *data=jeemiNetworkRequest(client,uid,pid,(char *)request.UTF8String);
                    xpc_dictionary_set_string(reply,"response",data);
                    free(data);
                    xpc_connection_send_message(peer,reply);
                });
            });
            xpc_connection_resume(peer);
        });
        xpc_connection_resume(listener);
        dispatch_main();
    }
}
