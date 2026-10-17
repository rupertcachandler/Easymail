//go:build darwin

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

static void darwinRequestAttentionNative(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (NSApp) {
			// Critical: the dock icon keeps bouncing until the user
			// activates the app — the "flashing sign on the icon".
			[NSApp requestUserAttention:NSCriticalRequest];
		}
	});
}
*/
import "C"

// platformNotify bounces the macOS dock icon until the user activates the
// app. It is a no-op while the app already has focus (AppKit ignores the
// request for the active application), so foreground arrivals just get the
// frontend chime.
func platformNotify(title, text string) {
	_ = title
	_ = text
	C.darwinRequestAttentionNative()
}
