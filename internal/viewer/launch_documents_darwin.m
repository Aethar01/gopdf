#import <Cocoa/Cocoa.h>
#import <CoreServices/CoreServices.h>
#include <stdlib.h>
#include <string.h>

// Finder and `open` hand a launching app its documents in an open-documents
// Apple Event, not as arguments. SDL finishes launching NSApp before it becomes
// the app delegate, so the event can reach AppKit before SDL can turn it into a
// drop. These handlers note the launch event and its files on their way to
// AppKit's own handlers, which still run as before.

typedef struct {
    AEEventHandlerUPP handler;
    SRefCon refcon;
} GoPDFAppleEventHandler;

static GoPDFAppleEventHandler gopdfAppKitOpenApplication;
static GoPDFAppleEventHandler gopdfAppKitOpenDocuments;

// gopdfLaunchPaths collects the launch documents until they are taken.
static NSMutableArray<NSString *> *gopdfLaunchPaths;
static BOOL gopdfLaunchEventHandled;

static OSErr gopdfForwardAppleEvent(GoPDFAppleEventHandler appKit, const AppleEvent *event, AppleEvent *reply) {
    if (appKit.handler == NULL) {
        return noErr;
    }
    return InvokeAEEventHandlerUPP(event, reply, appKit.refcon, appKit.handler);
}

static void gopdfRecordDocuments(const AppleEvent *event) {
    AEDesc copy;
    if (AEDuplicateDesc(event, &copy) != noErr) {
        return;
    }
    @autoreleasepool {
        // The descriptor owns the copy and disposes of it when released.
        NSAppleEventDescriptor *descriptor = [[NSAppleEventDescriptor alloc] initWithAEDescNoCopy:&copy];
        NSAppleEventDescriptor *files = [[descriptor paramDescriptorForKeyword:keyDirectObject] coerceToDescriptorType:typeAEList];
        for (NSInteger i = 1; i <= files.numberOfItems; i++) {
            NSString *path = [[files descriptorAtIndex:i] fileURLValue].path;
            if (path.length > 0) {
                [gopdfLaunchPaths addObject:path];
            }
        }
        [descriptor release];
    }
}

static OSErr gopdfHandleOpenApplication(const AppleEvent *event, AppleEvent *reply, SRefCon refcon) {
    (void)refcon;
    gopdfLaunchEventHandled = YES;
    return gopdfForwardAppleEvent(gopdfAppKitOpenApplication, event, reply);
}

static OSErr gopdfHandleOpenDocuments(const AppleEvent *event, AppleEvent *reply, SRefCon refcon) {
    (void)refcon;
    if (gopdfLaunchPaths != nil) {
        gopdfRecordDocuments(event);
    }
    gopdfLaunchEventHandled = YES;
    return gopdfForwardAppleEvent(gopdfAppKitOpenDocuments, event, reply);
}

static void gopdfWrapAppleEventHandler(AEEventID eventID, AEEventHandlerProcPtr wrapper, GoPDFAppleEventHandler *appKit) {
    if (AEGetEventHandler(kCoreEventClass, eventID, &appKit->handler, &appKit->refcon, false) != noErr) {
        *appKit = (GoPDFAppleEventHandler){0};
    }
    AEInstallEventHandler(kCoreEventClass, eventID, NewAEEventHandlerUPP(wrapper), NULL, false);
}

// gopdfWatchLaunch must run once, before SDL creates NSApp: AppKit installs its
// handlers just before it announces it will finish launching, and dispatches
// the launch event after.
void gopdfWatchLaunch(void) {
    gopdfLaunchPaths = [[NSMutableArray alloc] init];
    @autoreleasepool {
        NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
        [center addObserverForName:NSApplicationWillFinishLaunchingNotification
                            object:nil
                             queue:nil
                        usingBlock:^(NSNotification *notification) {
            (void)notification;
            gopdfWrapAppleEventHandler(kAEOpenApplication, gopdfHandleOpenApplication, &gopdfAppKitOpenApplication);
            gopdfWrapAppleEventHandler(kAEOpenDocuments, gopdfHandleOpenDocuments, &gopdfAppKitOpenDocuments);
        }];
        [center addObserverForName:NSApplicationDidFinishLaunchingNotification
                            object:nil
                             queue:nil
                        usingBlock:^(NSNotification *notification) {
            (void)notification;
            gopdfLaunchEventHandled = YES;
        }];
    }
}

// gopdfLaunchHandled reports whether the launch event has been handled, so its
// documents, if any, are known.
int gopdfLaunchHandled(void) {
    return gopdfLaunchEventHandled;
}

// gopdfTakeLaunchDocuments returns the launch documents and stops recording;
// later documents reach the viewer as SDL drops. The caller frees each path
// and the array.
char **gopdfTakeLaunchDocuments(int *count) {
    NSUInteger n = gopdfLaunchPaths.count;
    char **paths = n > 0 ? calloc(n, sizeof(char *)) : NULL;
    @autoreleasepool {
        for (NSUInteger i = 0; paths != NULL && i < n; i++) {
            paths[i] = strdup(gopdfLaunchPaths[i].fileSystemRepresentation);
        }
    }
    *count = paths != NULL ? (int)n : 0;
    [gopdfLaunchPaths release];
    gopdfLaunchPaths = nil;
    return paths;
}
