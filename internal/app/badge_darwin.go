//go:build darwin

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

static void darwinSetBadgeNative(long count) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (NSApp) {
			if (count > 0) {
				[[NSApp dockTile] setBadgeLabel:[NSString stringWithFormat:@"%ld", count]];
			} else {
				[[NSApp dockTile] setBadgeLabel:nil];
			}
		}
	});
}
*/
import "C"

// badgeSet shows the unread count as the classic red macOS dock badge,
// cleared when count is zero.
func (a *App) badgeSet(count int) {
	C.darwinSetBadgeNative(C.long(count))
}
