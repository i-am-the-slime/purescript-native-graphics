//go:build darwin

package graphics

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// dyld-load-time constructor: bootstrap NSApplication so ebiten's
// internal/ui package init (which runs before any of our Go init funcs,
// since ffi/player imports ebiten) can read NSScreen / NSEvent without
// crashing on a nil primary monitor.  The previous AppKit player
// transitively got this for free via ObjC class +load on its custom
// NSWindow/NSView subclasses; once those were deleted nothing was
// triggering the Cocoa runtime at load time.
__attribute__((constructor))
static void mg_bootstrapAppKit(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
    }
}

static double mg_deviceScaleFactor(void) {
    NSScreen *screen = [NSScreen mainScreen];
    if (screen == nil) return 1.0;
    return (double)[screen backingScaleFactor];
}
*/
import "C"

func deviceScaleFactor() float64 {
	return float64(C.mg_deviceScaleFactor())
}
