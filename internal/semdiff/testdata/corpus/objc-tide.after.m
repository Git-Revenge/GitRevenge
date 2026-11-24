#import <Foundation/Foundation.h>
@implementation Tide
- (int)height:(int)at {
    int value = at % 48;
    NSLog(@"height now %d", value);
    return value;
}
@end
