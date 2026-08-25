#import <Cocoa/Cocoa.h>

static void gopdfApplyMacOSWindowStyle(NSWindow *window) {
    if (window == nil) {
        return;
    }

    window.titleVisibility = NSWindowTitleHidden;
    window.titlebarAppearsTransparent = YES;
    window.styleMask |= NSWindowStyleMaskFullSizeContentView;
}

void gopdfConfigureMacOSWindow(void *windowPointer) {
    NSWindow *window = (__bridge NSWindow *)windowPointer;
    if (window == nil) {
        return;
    }

    if ([NSThread isMainThread]) {
        gopdfApplyMacOSWindowStyle(window);
    } else {
        dispatch_async(dispatch_get_main_queue(), ^{
            gopdfApplyMacOSWindowStyle(window);
        });
    }
}
