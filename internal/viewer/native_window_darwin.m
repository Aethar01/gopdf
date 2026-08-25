#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

static const void *GoPDFTitlebarHoverControllerKey = &GoPDFTitlebarHoverControllerKey;

@interface GoPDFTitlebarDragView : NSView
@end

@implementation GoPDFTitlebarDragView

- (BOOL)isOpaque {
    return NO;
}

- (void)drawRect:(NSRect)dirtyRect {
    (void)dirtyRect;
}

- (void)mouseDown:(NSEvent *)event {
    [self.window performWindowDragWithEvent:event];
}

@end

@interface GoPDFTitlebarHoverController : NSObject {
    NSView *_trackingView;
    NSTrackingArea *_trackingArea;
    GoPDFTitlebarDragView *_dragView;
    NSButton *_buttons[3];
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

    _dragView = [[GoPDFTitlebarDragView alloc] initWithFrame:_trackingView.bounds];
    _dragView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [_trackingView addSubview:_dragView positioned:NSWindowBelow relativeTo:nil];

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

- (void)setTrafficLightsVisible:(BOOL)visible animated:(BOOL)animated {
    if (_visible == visible && animated) {
        return;
    }
    _visible = visible;

    if (!animated) {
        for (NSUInteger i = 0; i < 3; i++) {
            NSButton *button = _buttons[i];
            button.alphaValue = visible ? 1.0 : 0.0;
            button.hidden = !visible;
        }
        return;
    }

    if (visible) {
        for (NSUInteger i = 0; i < 3; i++) {
            NSButton *button = _buttons[i];
            button.alphaValue = 0.0;
            button.hidden = NO;
        }
    }

    [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
        context.duration = 0.16;
        context.allowsImplicitAnimation = YES;

        for (NSUInteger i = 0; i < 3; i++) {
            _buttons[i].animator.alphaValue = visible ? 1.0 : 0.0;
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
    [_dragView removeFromSuperview];
    [_dragView release];
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
