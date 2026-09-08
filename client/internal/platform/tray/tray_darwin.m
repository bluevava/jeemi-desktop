//go:build darwin

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include "tray_darwin.h"

@interface JeemiStatusTray : NSObject
@property(nonatomic) uintptr_t handle;
@property(nonatomic, strong) NSStatusItem *item;
@property(nonatomic, strong) NSMenu *menu;
@property(nonatomic) BOOL contextMenuEnabled;
- (void)click:(id)sender;
- (void)selectMenuItem:(NSMenuItem *)sender;
- (void)remove;
@end

@implementation JeemiStatusTray
- (void)click:(id)sender {
    if (self.item == nil) return;
    JeemiStatusTray *tray = self;
    NSEvent *event = [NSApp currentEvent];
    if (event.type == NSEventTypeRightMouseUp && self.contextMenuEnabled) {
        // Attach only while open so left-button double clicks remain available.
        tray.item.menu = tray.menu;
        [tray.item.button performClick:nil];
        tray.item.menu = nil;
    } else if (event.type == NSEventTypeLeftMouseUp && event.clickCount == 2) {
        jeemiTrayEvent(self.handle, JEEMI_TRAY_DOUBLE_CLICK, 0);
    }
}

- (void)selectMenuItem:(NSMenuItem *)sender {
    if (self.item == nil) return;
    jeemiTrayEvent(self.handle, JEEMI_TRAY_MENU, (uint32_t)sender.tag);
}

- (void)remove {
    self.item.button.target = nil;
    self.item.button.action = NULL;
    self.item.menu = nil;
    for (NSMenuItem *item in self.menu.itemArray) {
        item.target = nil;
        item.action = NULL;
    }
    [self.menu cancelTracking];
    if (self.item != nil) [[NSStatusBar systemStatusBar] removeStatusItem:self.item];
    self.item = nil;
    self.menu = nil;
}
@end

static JeemiStatusTray *activeTray;

// AppKit access runs on Wails' existing Cocoa main thread.
static void onMainThread(dispatch_block_t operation) {
    if ([NSThread isMainThread]) operation();
    else dispatch_sync(dispatch_get_main_queue(), operation);
}

static BOOL isActive(uintptr_t handle) {
    return activeTray != nil && activeTray.handle == handle;
}

void jeemi_tray_start(uintptr_t handle) {
    // Wails OnStartup is a Go goroutine and can precede the Cocoa event loop.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (activeTray != nil) {
            jeemiTrayEvent(handle, JEEMI_TRAY_EXIT, 0);
            return;
        }
        JeemiStatusTray *candidate = [[JeemiStatusTray alloc] init];
        candidate.handle = handle;
        @try {
            candidate.item = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
            candidate.menu = [[NSMenu alloc] init];
            candidate.menu.autoenablesItems = NO;
            candidate.item.button.target = candidate;
            candidate.item.button.action = @selector(click:);
            [candidate.item.button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
            if (candidate.item.button == nil) {
                [candidate remove];
                jeemiTrayEvent(handle, JEEMI_TRAY_EXIT, 0);
                return;
            }
            activeTray = candidate;
        } @catch (NSException *exception) {
            [candidate remove];
            jeemiTrayEvent(handle, JEEMI_TRAY_EXIT, 0);
            return;
        }
        jeemiTrayEvent(handle, JEEMI_TRAY_READY, 0);
    });
}

void jeemi_tray_stop(uintptr_t handle) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (isActive(handle)) {
            [activeTray remove];
            activeTray = nil;
            // Remove only our status item. Wails owns application termination.
            jeemiTrayEvent(handle, JEEMI_TRAY_EXIT, 0);
        }
    });
}

void jeemi_tray_set_icon(uintptr_t handle, const void *bytes, size_t length) {
    onMainThread(^{
        if (!isActive(handle)) return;
        NSData *data = [NSData dataWithBytes:bytes length:length];
        NSImage *image = [[NSImage alloc] initWithData:data];
        image.size = NSMakeSize(18, 18);
        activeTray.item.button.image = image;
        activeTray.item.button.imagePosition = NSImageOnly;
    });
}

void jeemi_tray_set_tooltip(uintptr_t handle, const char *text) {
    onMainThread(^{
        if (isActive(handle)) activeTray.item.button.toolTip = [NSString stringWithUTF8String:text];
    });
}

void jeemi_tray_enable_menu(uintptr_t handle) {
    onMainThread(^{
        if (isActive(handle)) activeTray.contextMenuEnabled = YES;
    });
}

void jeemi_tray_update_item(uintptr_t handle, uint32_t identifier, const char *title, const char *tooltip) {
    onMainThread(^{
        if (!isActive(handle)) return;
        NSMenuItem *item = [activeTray.menu itemWithTag:(NSInteger)identifier];
        if (item == nil) {
            item = [[NSMenuItem alloc] initWithTitle:@"" action:@selector(selectMenuItem:) keyEquivalent:@""];
            item.tag = (NSInteger)identifier;
            item.target = activeTray;
            [activeTray.menu addItem:item];
        }
        item.title = [NSString stringWithUTF8String:title];
        item.toolTip = [NSString stringWithUTF8String:tooltip];
    });
}

void jeemi_tray_add_separator(uintptr_t handle) {
    onMainThread(^{
        if (isActive(handle)) [activeTray.menu addItem:[NSMenuItem separatorItem]];
    });
}
