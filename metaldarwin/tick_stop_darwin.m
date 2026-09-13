#import <Foundation/Foundation.h>

// Owned here (not in appkit_darwin.go's cgo preamble) because cgo splices
// the preamble into multiple generated .c files; a non-static definition
// would produce duplicate _mg_stop_tick / _g_tickTimer symbols at link.
NSTimer *g_tickTimer = nil;

// Idempotent, main-thread timer teardown before returning from native run.
void mg_stop_tick(void) {
    if (![NSThread isMainThread]) {
        dispatch_sync(dispatch_get_main_queue(), ^{ mg_stop_tick(); });
        return;
    }
    if (g_tickTimer != nil) {
        [g_tickTimer invalidate];
        g_tickTimer = nil;
    }
}
