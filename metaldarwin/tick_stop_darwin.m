#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

extern void goRenderTick(void);
extern int mg_quit_requested;

@interface MgTickTarget : NSObject
- (void)tick:(id)sender;
- (void)screenChanged:(NSNotification *)notification;
@end

// Owned out-of-line: cgo duplicates appkit_darwin.go's preamble.
static NSTimer *g_tickTimer = nil;
static id g_displayLink = nil;
static MgTickTarget *g_tickTarget = nil;
static NSWindow *g_tickWindow = nil;
static int g_requestedFPS = 60;
static BOOL g_tickRunning = NO;
static BOOL g_insideTick = NO;
static NSUInteger g_tickGeneration = 0;

int mg_screen_maximum_fps(NSScreen *screen) {
    if (@available(macOS 12.0, *)) {
        NSInteger maximum = screen.maximumFramesPerSecond;
        if (maximum > 0) return (int)maximum;
    }
    return 60;
}

static void mg_invalidate_tick_source(void) {
    if (@available(macOS 14.0, *)) {
        [(CADisplayLink *)g_displayLink invalidate];
        g_displayLink = nil;
    }
    [g_tickTimer invalidate];
    g_tickTimer = nil;
}

void mg_refresh_pacing(void) {
    if (!g_tickRunning) return;
    mg_invalidate_tick_source();
    NSScreen *screen = g_tickWindow.screen ?: NSScreen.mainScreen;
    int fps = MIN(g_requestedFPS, mg_screen_maximum_fps(screen));
    if (@available(macOS 14.0, *)) {
        if (screen != nil) {
            CADisplayLink *link = [screen displayLinkWithTarget:g_tickTarget selector:@selector(tick:)];
            link.preferredFrameRateRange = CAFrameRateRangeMake(fps, fps, fps);
            g_displayLink = link;
            [link addToRunLoop:NSRunLoop.mainRunLoop forMode:NSRunLoopCommonModes];
            return;
        }
    }
    // Older systems (or no attached screen) keep a rate-clamped timer fallback.
    g_tickTimer = [NSTimer timerWithTimeInterval:1.0/(double)fps
                                        target:g_tickTarget selector:@selector(tick:)
                                      userInfo:nil repeats:YES];
    [NSRunLoop.mainRunLoop addTimer:g_tickTimer forMode:NSRunLoopCommonModes];
}

void mg_dispatch_tick(id sender) {
    if (!g_tickRunning || g_insideTick) return;
    // An invalidated source must not dispatch into a new pacing generation.
    if (sender != nil && sender != g_tickTimer && sender != g_displayLink) return;
    @autoreleasepool {
        g_insideTick = YES;
        @try {
            goRenderTick();
        } @finally {
            g_insideTick = NO;
        }
    }
}

// A minimized/occluded display link need not tick. Deliver the host's close
// event independently, on the main run loop, without re-entering a callback.
void mg_dispatch_close_request(void) {
    NSUInteger generation = g_tickGeneration;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (generation == g_tickGeneration) mg_dispatch_tick(nil);
    });
}

// Idempotent main-thread teardown precedes renderer/resource cleanup.
void mg_stop_tick(void) {
    if (![NSThread isMainThread]) {
        dispatch_sync(dispatch_get_main_queue(), ^{ mg_stop_tick(); });
        return;
    }
    g_tickRunning = NO;
    ++g_tickGeneration;
    mg_invalidate_tick_source();
    if (g_tickTarget != nil) [NSNotificationCenter.defaultCenter removeObserver:g_tickTarget];
    g_tickTarget = nil;
    g_tickWindow = nil;
}

void mg_start_pacing(NSWindow *window, int fps) {
    NSCAssert(NSThread.isMainThread, @"AppKit pacing must start on the main thread");
    mg_stop_tick();
    mg_quit_requested = 0;
    g_tickWindow = window;
    g_requestedFPS = fps > 0 ? fps : 60;
    g_tickTarget = [[MgTickTarget alloc] init];
    g_tickRunning = YES;
    NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
    [center addObserver:g_tickTarget selector:@selector(screenChanged:)
                   name:NSWindowDidChangeScreenNotification object:window];
    [center addObserver:g_tickTarget selector:@selector(screenChanged:)
                   name:NSApplicationDidChangeScreenParametersNotification object:nil];
    mg_refresh_pacing();
}
