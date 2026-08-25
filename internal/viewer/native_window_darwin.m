#import <Cocoa/Cocoa.h>

static void gopdfSetTrafficLightsHidden(NSWindow *window, BOOL hidden) {
    [[window standardWindowButton:NSWindowCloseButton] setHidden:hidden];
    [[window standardWindowButton:NSWindowMiniaturizeButton] setHidden:hidden];
    [[window standardWindowButton:NSWindowZoomButton] setHidden:hidden];
}

@interface GoPDFTitlebarHoverView : NSView {
    NSTrackingArea *_trackingArea;
}
@end

@implementation GoPDFTitlebarHoverView

- (NSView *)hitTest:(NSPoint)point {
    (void)point;
    return nil;
}

- (void)updateTrackingAreas {
    [super updateTrackingAreas];

    if (_trackingArea != nil) {
        [self removeTrackingArea:_trackingArea];
        [_trackingArea release];
        _trackingArea = nil;
    }

    _trackingArea = [[NSTrackingArea alloc]
        initWithRect:self.bounds
        options:(NSTrackingMouseEnteredAndExited |
                 NSTrackingActiveAlways |
                 NSTrackingInVisibleRect)
        owner:self
        userInfo:nil];
    [self addTrackingArea:_trackingArea];
}

- (void)mouseEntered:(NSEvent *)event {
    (void)event;
    gopdfSetTrafficLightsHidden(self.window, NO);
}

- (void)mouseExited:(NSEvent *)event {
    (void)event;
    gopdfSetTrafficLightsHidden(self.window, YES);
}

- (void)dealloc {
    if (_trackingArea != nil) {
        [self removeTrackingArea:_trackingArea];
        [_trackingArea release];
    }
    [super dealloc];
}

@end

static void gopdfApplyMacOSWindowStyle(NSWindow *window) {
    if (window == nil) {
        return;
    }

    window.titleVisibility = NSWindowTitleHidden;
    window.titlebarAppearsTransparent = YES;
    window.styleMask |= NSWindowStyleMaskFullSizeContentView;

    NSView *contentView = window.contentView;
    if (contentView == nil) {
        return;
    }

    const CGFloat hoverHeight = 44.0;
    NSRect bounds = contentView.bounds;
    NSRect hoverFrame = NSMakeRect(
        NSMinX(bounds),
        NSMaxY(bounds) - hoverHeight,
        NSWidth(bounds),
        hoverHeight
    );

    GoPDFTitlebarHoverView *hoverView = [[GoPDFTitlebarHoverView alloc] initWithFrame:hoverFrame];
    hoverView.autoresizingMask = NSViewWidthSizable | NSViewMinYMargin;
    [contentView addSubview:hoverView positioned:NSWindowAbove relativeTo:nil];
    [hoverView release];

    gopdfSetTrafficLightsHidden(window, YES);
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
