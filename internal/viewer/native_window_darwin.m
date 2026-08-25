#import <Cocoa/Cocoa.h>
#import <SDL3/SDL.h>

static void gopdfApplyMacOSWindowStyle(NSWindow *window) {
    if (window == nil) {
        return;
    }

    window.titleVisibility = NSWindowTitleHidden;
    window.titlebarAppearsTransparent = YES;
    window.styleMask |= NSWindowStyleMaskFullSizeContentView;
}

void gopdfConfigureMacOSWindow(void *windowPointer) {
    SDL_Window *sdlWindow = (SDL_Window *)windowPointer;
    if (sdlWindow == NULL) {
        return;
    }

    SDL_PropertiesID properties = SDL_GetWindowProperties(sdlWindow);
    NSWindow *window = (__bridge NSWindow *)SDL_GetPointerProperty(
        properties,
        SDL_PROP_WINDOW_COCOA_WINDOW_POINTER,
        NULL
    );
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
