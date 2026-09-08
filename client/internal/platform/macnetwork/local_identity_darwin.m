//go:build darwin && jeemi_local_test

#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include "network_darwin.h"

#define LOCAL_BASE "/Library/Application Support/JeemiNetworkHelperLocalTest"
#define LOCAL_HELPER "/Library/PrivilegedHelperTools/jeemi-authorizer"
#define LEGACY_HELPER "/Library/PrivilegedHelperTools/" JEEMI_NETWORK_SERVICE
#define LOCAL_PLIST "/Library/LaunchDaemons/" JEEMI_NETWORK_SERVICE ".plist"

// Local testing uses an administrator-owned allowlist of exact build CDHashes.
// It never adds a CA to the keychain or weakens the release identity requirement.
static NSData *protectedFile(const char *name) {
    char resolved[PATH_MAX];
    if(realpath(name,resolved)==NULL || strcmp(name,resolved)!=0)return nil;
    NSString *path=[NSString stringWithUTF8String:name];
    NSString *parent=path.stringByDeletingLastPathComponent;
    struct stat st;
    while(YES){
        if(lstat(parent.fileSystemRepresentation,&st)!=0 || !S_ISDIR(st.st_mode) || st.st_uid!=0 || (st.st_mode&022)!=0)return nil;
        if([parent isEqualToString:@"/"])break;
        parent=parent.stringByDeletingLastPathComponent;
    }
    int fd=open(name,O_RDONLY|O_NOFOLLOW|O_CLOEXEC);
    if(fd<0)return nil;
    if(fstat(fd,&st)!=0 || !S_ISREG(st.st_mode) || st.st_uid!=0 || st.st_nlink!=1 || (st.st_mode&022)!=0 || st.st_size<1 || st.st_size>16384){close(fd);return nil;}
    NSMutableData *data=[NSMutableData dataWithLength:(NSUInteger)st.st_size];
    ssize_t count=read(fd,data.mutableBytes,data.length);close(fd);
    return count==(ssize_t)data.length ? data : nil;
}
static NSDictionary *trust(void) {
    NSData *data=protectedFile(LOCAL_BASE "/trust.json");
    if(data==nil)return nil;
    NSDictionary *value=[NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    if(![value isKindOfClass:NSDictionary.class] || value.count!=4 || ![value[@"version"] isEqual:@1] || ![value[@"uid"] isKindOfClass:NSNumber.class] || [value[@"uid"] unsignedLongLongValue]<500 || [value[@"uid"] unsignedLongLongValue]>UINT32_MAX)return nil;
    NSCharacterSet *invalid=[[NSCharacterSet characterSetWithCharactersInString:@"0123456789abcdef"] invertedSet];
    for(NSString *key in @[@"gui",@"helper"]){
        NSArray *hashes=value[key];
        if(![hashes isKindOfClass:NSArray.class] || hashes.count<1 || hashes.count>2)return nil;
        for(id hash in hashes)if(![hash isKindOfClass:NSString.class] || [hash length]!=40 || [hash rangeOfCharacterFromSet:invalid].location!=NSNotFound)return nil;
        if([NSSet setWithArray:hashes].count!=hashes.count)return nil;
    }
    return value;
}
static NSString *rule(NSDictionary *value,const char *identifier){
    if(value==nil)return nil;
    NSString *key=strcmp(identifier,JEEMI_GUI_IDENTIFIER)==0 ? @"gui" : (strcmp(identifier,JEEMI_NETWORK_SERVICE)==0 ? @"helper" : nil);
    if(key==nil)return nil;
    NSArray *hashes=value[key];
    NSMutableArray *clauses=[NSMutableArray arrayWithCapacity:hashes.count];
    for(NSString *hash in hashes)[clauses addObject:[NSString stringWithFormat:@"cdhash H\"%@\"",hash]];
    return [NSString stringWithFormat:@"identifier \"%s\" and (%@)",identifier,[clauses componentsJoinedByString:@" or "]];
}
static BOOL check(NSURL *url,NSString *ruleText){
    SecStaticCodeRef code=NULL;SecRequirementRef requirement=NULL;
    if(ruleText==nil || SecStaticCodeCreateWithPath((__bridge CFURLRef)url,0,&code)!=errSecSuccess)return NO;
    OSStatus status=SecRequirementCreateWithString((__bridge CFStringRef)ruleText,0,&requirement);
    if(status==errSecSuccess)status=SecStaticCodeCheckValidity(code,kSecCSStrictValidate|kSecCSCheckAllArchitectures,requirement);
    if(requirement)CFRelease(requirement);CFRelease(code);
    return status==errSecSuccess;
}
static BOOL ownAllowed(NSDictionary *value){
    SecCodeRef code=NULL;SecStaticCodeRef staticCode=NULL;CFDictionaryRef info=NULL;
    if(SecCodeCopySelf(0,&code)!=errSecSuccess)return NO;
    OSStatus status=SecCodeCopyStaticCode(code,0,&staticCode);CFRelease(code);
    if(status!=errSecSuccess)return NO;
    status=SecCodeCopySigningInformation(staticCode,kSecCSSigningInformation,&info);CFRelease(staticCode);
    if(status!=errSecSuccess)return NO;
    NSDictionary *details=CFBridgingRelease(info);
    NSString *identifier=details[(__bridge id)kSecCodeInfoIdentifier];
    NSData *hash=details[(__bridge id)kSecCodeInfoUnique];
    if(![identifier isKindOfClass:NSString.class] || ![hash isKindOfClass:NSData.class] || hash.length!=20)return NO;
    NSMutableString *hex=[NSMutableString string];const unsigned char *bytes=hash.bytes;
    for(NSUInteger i=0;i<hash.length;i++)[hex appendFormat:@"%02x",bytes[i]];
    NSString *key=[identifier isEqualToString:@JEEMI_GUI_IDENTIFIER]?@"gui":([identifier isEqualToString:@JEEMI_NETWORK_SERVICE]?@"helper":nil);
    return key!=nil && [value[key] containsObject:hex];
}
const char *jeemi_local_requirement(const char *identifier){
    @autoreleasepool {
        NSDictionary *value=trust();
        if(!ownAllowed(value))return NULL;
        NSString *text=rule(value,identifier);
        return text ? strdup(text.UTF8String) : NULL;
    }
}
uint32_t jeemi_local_uid(void){@autoreleasepool{return [trust()[@"uid"] unsignedIntValue];}}
char *jeemi_local_facts(void){
    @autoreleasepool {
        NSURL *app=NSBundle.mainBundle.bundleURL;
        NSURL *bundled=[app URLByAppendingPathComponent:@"Contents/MacOS/jeemi-authorizer"];
        BOOL packaged=[NSFileManager.defaultManager isExecutableFileAtPath:bundled.path];
        BOOL signedCode=check(app,@"identifier \"" JEEMI_GUI_IDENTIFIER "\"") && check(bundled,@"identifier \"" JEEMI_NETWORK_SERVICE "\"");
        NSDictionary *value=trust();
        NSURL *installedURL=[NSURL fileURLWithPath:@LOCAL_HELPER];
        struct stat st;
        BOOL protected=lstat(LOCAL_HELPER,&st)==0 && S_ISREG(st.st_mode) && st.st_uid==0 && (st.st_mode&022)==0 && st.st_nlink==1;
        BOOL installed=protected && ownAllowed(value) && check(app,rule(value,JEEMI_GUI_IDENTIFIER)) && check(installedURL,rule(value,JEEMI_NETWORK_SERVICE)) && [value[@"uid"] unsignedIntValue]==getuid();
        NSData *plistData=protectedFile(LOCAL_PLIST);
        NSDictionary *plist=plistData ? [NSPropertyListSerialization propertyListWithData:plistData options:0 format:NULL error:nil] : nil;
        BOOL registered=[plist isKindOfClass:NSDictionary.class] && [plist[@"Label"] isEqual:@JEEMI_NETWORK_SERVICE] && [plist[@"ProgramArguments"] isEqual:@[@LOCAL_HELPER]];
        // lstat also detects broken links and partial installs. Never report
        // these as absent just because the executable cannot be trusted/run.
        BOOL present=lstat(LOCAL_HELPER,&st)==0 || lstat(LEGACY_HELPER,&st)==0 || lstat(LOCAL_PLIST,&st)==0 || lstat(LOCAL_BASE "/trust.json",&st)==0;
        // Root-only runtime records cannot be inspected by the GUI. A fixed
        // root-owned receipt records successful cleanup of an otherwise orphaned
        // runtime directory; installation removes the receipt before activation.
        NSData *clean=protectedFile(LOCAL_BASE "/cleanup-complete");
        BOOL cleaned=[clean isEqualToData:[@"Jeemi authorization cleanup v1\n" dataUsingEncoding:NSUTF8StringEncoding]];
        if(lstat(LOCAL_BASE "/runtime",&st)==0 && !cleaned)present=YES;
        NSDictionary *facts=@{@"supported":@YES,@"localTest":@YES,@"packaged":@(packaged),@"signed":@(signedCode),@"installed":@(installed),@"present":@(present),@"service":registered?@"enabled":@"not_registered"};
        NSData *data=[NSJSONSerialization dataWithJSONObject:facts options:0 error:nil];
        return data ? strdup([[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding].UTF8String) : NULL;
    }
}
