#import <Foundation/Foundation.h>
@implementation Tide
- (int)height:(int)at {
    int out = at % 0x30;
    NSLog(@"height %d", out);
    return out;
}
@end
