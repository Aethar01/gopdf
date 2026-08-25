#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

static const void *GoPDFTitlebarHoverControllerKey = &GoPDFTitlebarHoverControllerKey;

@interface GoPDFTitlebarHoverController : NSObject {
    NSWindow *_window;
    NSView *_trackingView;
    NSTrackingArea *_trackingArea;
    NSButton *_buttons[3];
    NSRect _shownFrames[3];
    BOOL _visible;
}

- (instancetype)initWithWindow:(NSWindow *)window;

@end

@implementation GoPDFTitlebarHoverController

- (instancetype)initWithWindow:(NSWindow *)window {
    self = [super init];
    if (self == nil) {
        return nil;
    }

    _window = window;
    _buttons[0] = [[window standardWindowButton:NSWindowCloseButton] retain];
    _buttons[1] = [[window standardWindowButton:NSWindowMiniaturizeButton] retain];
    _buttons[2] = [[window standardWindowButton:NSWindowZoomButton] retain];

    if (_buttons[0] == nil || _buttons[1] == nil || _buttons[2] == nil) {
        [self release];
        return nil;
    }

    _trackingView = [[_buttons[0] superview] retain];
    if (_trackingView == nil) {
        [self release];
        return nil;
    }

    for (NSUInteger i = 0; i < 3; i++) {
        _shownFrames[i] = _buttons[i].frame;
    }

    _trackingArea = [[NSTrackingArea alloc]
        initWithRect:NSZeroRect
        options:(NSTrackingMouseEnteredAndExited |
                 NSTrackingActiveAlways |
                 NSTrackingInVisibleRect)
        owner:self
        userInfo:nil];
    [_trackingView addTrackingArea:_trackingArea];

    [self setTrafficLightsVisible:NO animated:NO];
    return self;
}

- (NSRect)hiddenFrameForButtonAtIndex:(NSUInteger)index {
    NSButton *button = _buttons[index];
    NSView *superview = button.superview;
    NSRect frame = _shownFrames[index];

    if (superview.isFlipped) {
        frame.origin.y = NSMinY(superview.bounds) - NSHeight(frame) - 2.0;
    } else {
        frame.origin.y = NSMaxY(superview.bounds) + 2.0;
    }
    return frame;
}

- (void)setTrafficLightsVisible:(BOOL)visible animated:(BOOL)animated {
    if (_visible == visible && animated) {
        return;
    }
    _visible = visible;

    if (!animated) {
        for (NSUInteger i = 0; i < 3; i++) {
            NSButton *button = _buttons[i];
            if (visible) {
                button.frame = _shownFrames[i];
                button.alphaValue = 1.0;
                button.hidden = NO;
            } else {
                button.frame = [self hiddenFrameForButtonAtIndex:i];
                button.alphaValue = 0.0;
                button.hidden = YES;
            }
        }
        return;
    }

    if (visible) {
        for (NSUInteger i = 0; i < 3; i++) {
            NSButton *button = _buttons[i];
            button.frame = [self hiddenFrameForButtonAtIndex:i];
            button.alphaValue = 0.0;
            button.hidden = NO;
        }
    }

    [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
        context.duration = 0.16;
        context.allowsImplicitAnimation = YES;

        for (NSUInteger i = 0; i < 3; i++) {
            NSButton *button = _buttons[i];
            if (visible) {
                button.animator.frame = _shownFrames[i];
                button.animator.alphaValue = 1.0;
            } else {
                button.animator.frame = [self hiddenFrameForButtonAtIndex:i];
                button.animator.alphaValue = 0.0;
            }
        }
    } completionHandler:^{
        if (!_visible) {
            for (NSUInteger i = 0; i < 3; i++) {
                _buttons[i].hidden = YES;
            }
        }
    }];
}

- (void)mouseEntered:(NSEvent *)event {
    (void)event;
    [self setTrafficLightsVisible:YES animated:YES];
}

- (void)mouseExited:(NSEvent *)event {
    (void)event;
    [self setTrafficLightsVisible:NO animated:YES];
}

- (void)dealloc {
    if (_trackingArea != nil && _trackingView != nil) {
        [_trackingView removeTrackingArea:_trackingArea];
    }
    [_trackingArea release];
    [_trackingView release];
    for (NSUInteger i = 0; i < 3; i++) {
        [_buttons[i] release];
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

    GoPDFTitlebarHoverController *controller =
        [[GoPDFTitlebarHoverController alloc] initWithWindow:window];
    if (controller != nil) {
        objc_setAssociatedObject(
            window,
            GoPDFTitlebarHoverControllerKey,
            controller,
            OBJC_ASSOCIATION_RETAIN_NONATOMIC
        );
        [controller release];
    }
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
