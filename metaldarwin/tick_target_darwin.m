// Build only on darwin; cgo picks up .m files automatically.
#import <Foundation/Foundation.h>

extern void goRenderTick(void);

@interface MgTickTarget : NSObject
- (void)tick:(id)sender;
@end

@implementation MgTickTarget
- (void)tick:(id)sender { (void)sender; goRenderTick(); }
@end
