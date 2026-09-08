//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

extern void jeemiSessionEnd(void);
static Class delegateClass;
static IMP originalTerminate;
static BOOL pending;

void jeemi_session_reply(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!pending) return;
        pending = NO;
        [NSApp replyToApplicationShouldTerminate:YES];
    });
}

// Wails v2.14's delegate unconditionally returns NSTerminateCancel and sends
// the same Go message as a window close. Adapt only this method, keeping its
// delegate object and all other Wails callbacks intact.
static NSApplicationTerminateReply shouldTerminate(id self, SEL selector, NSApplication *app) {
    if (!pending) {
        pending = YES;
        jeemiSessionEnd();
        // Even a stuck Go cleanup must never prevent a system restart.
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 4500*NSEC_PER_MSEC), dispatch_get_main_queue(), ^{jeemi_session_reply();});
    }
    return NSTerminateLater;
}

void jeemi_session_watch_start(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (delegateClass != Nil) return;
        Class cls = object_getClass(NSApp.delegate);
        SEL selector = @selector(applicationShouldTerminate:);
        Method method = class_getInstanceMethod(cls, selector);
        if (method == NULL) return;
        delegateClass = cls;
        originalTerminate = method_getImplementation(method);
        class_replaceMethod(cls, selector, (IMP)shouldTerminate, method_getTypeEncoding(method));
    });
}

void jeemi_session_watch_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (delegateClass == Nil) return;
        SEL selector = @selector(applicationShouldTerminate:);
        Method method = class_getInstanceMethod(delegateClass, selector);
        class_replaceMethod(delegateClass, selector, originalTerminate, method_getTypeEncoding(method));
        delegateClass = Nil;
    });
}
