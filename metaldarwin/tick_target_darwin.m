// Build only on darwin; cgo picks up .m files automatically.
#import <Foundation/Foundation.h>

extern void mg_dispatch_tick(id sender);
extern void mg_refresh_pacing(void);

@interface MgTickTarget : NSObject
- (void)tick:(id)sender;
- (void)screenChanged:(NSNotification *)notification;
@end

@implementation MgTickTarget
- (void)tick:(id)sender { mg_dispatch_tick(sender); }
- (void)screenChanged:(NSNotification *)notification { (void)notification; mg_refresh_pacing(); }
@end
