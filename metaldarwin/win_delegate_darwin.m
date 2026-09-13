// Out-of-line implementations keep Objective-C symbols unique under cgo.
#import <Cocoa/Cocoa.h>

extern void mg_stop_tick(void);
int mg_quit_requested = 0;

void mg_request_stop(void) {
    if (![NSThread isMainThread]) {
        dispatch_async(dispatch_get_main_queue(), ^{ mg_request_stop(); });
        return;
    }
    mg_stop_tick();
    mg_quit_requested = 1;
    [NSApp stop:nil];
    // Wake nextEventMatchingMask so the application run loop can return.
    NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
                                      location:NSZeroPoint modifierFlags:0 timestamp:0
                                  windowNumber:0 context:nil subtype:0 data1:0 data2:0];
    [NSApp postEvent:event atStart:YES];
}

@interface MgWinDelegate : NSObject <NSWindowDelegate>
@end
@implementation MgWinDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender { (void)sender; mg_quit_requested = 1; return NO; }
@end

@interface MgAppDelegate : NSObject <NSApplicationDelegate>
@end
@implementation MgAppDelegate
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender { (void)sender; return NO; }
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    (void)sender;
    mg_quit_requested = 1;
    return NSTerminateCancel;
}
- (void)applicationWillTerminate:(NSNotification *)note { (void)note; mg_stop_tick(); }
@end

@interface MgNoDragView : NSView
@end
@implementation MgNoDragView
- (BOOL)mouseDownCanMoveWindow { return NO; }
// The event monitor reports these events; consuming them prevents default beeps.
- (void)mouseDown:(NSEvent *)event { (void)event; }
- (void)mouseUp:(NSEvent *)event { (void)event; }
- (void)mouseDragged:(NSEvent *)event { (void)event; }
@end
