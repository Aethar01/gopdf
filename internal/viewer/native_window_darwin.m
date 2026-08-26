#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

static const void *GoPDFTitlebarHoverControllerKey = &GoPDFTitlebarHoverControllerKey;

static void gopdfApplyMacOSWindowStyleProperties(NSWindow *window) {
    if (window == nil) {
        return;
    }

    window.titleVisibility = NSWindowTitleHidden;
    window.titlebarAppearsTransparent = YES;
    window.styleMask |= NSWindowStyleMaskFullSizeContentView;
}

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
    NSWindow *_window;
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

    _window = window;
    if (![self rebuildTitlebarViews]) {
        [self release];
        return nil;
    }

    [[NSNotificationCenter defaultCenter]
        addObserver:self
        selector:@selector(windowDidExitFullScreen:)
        name:NSWindowDidExitFullScreenNotification
        object:window];

    return self;
}

- (void)tearDownTitlebarViews {
    if (_trackingArea != nil && _trackingView != nil) {
        [_trackingView removeTrackingArea:_trackingArea];
    }

    [_dragView removeFromSuperview];
    [_dragView release];
    _dragView = nil;

    [_trackingArea release];
    _trackingArea = nil;

    [_trackingView release];
    _trackingView = nil;

    for (NSUInteger i = 0; i < 3; i++) {
        [_buttons[i] release];
        _buttons[i] = nil;
    }
}

- (BOOL)rebuildTitlebarViews {
    [self tearDownTitlebarViews];

    _buttons[0] = [[_window standardWindowButton:NSWindowCloseButton] retain];
    _buttons[1] = [[_window standardWindowButton:NSWindowMiniaturizeButton] retain];
    _buttons[2] = [[_window standardWindowButton:NSWindowZoomButton] retain];

    if (_buttons[0] == nil || _buttons[1] == nil || _buttons[2] == nil) {
        [self tearDownTitlebarViews];
        return NO;
    }

    _trackingView = [[_buttons[0] superview] retain];
    if (_trackingView == nil) {
        [self tearDownTitlebarViews];
        return NO;
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
    return YES;
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

- (void)windowDidExitFullScreen:(NSNotification *)notification {
    (void)notification;

    gopdfApplyMacOSWindowStyleProperties(_window);
    [self rebuildTitlebarViews];
}

- (void)dealloc {
    [[NSNotificationCenter defaultCenter] removeObserver:self];
    [self tearDownTitlebarViews];
    [super dealloc];
}

@end

static void gopdfApplyMacOSWindowStyle(NSWindow *window) {
    if (window == nil) {
        return;
    }

    gopdfApplyMacOSWindowStyleProperties(window);

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
