#import <Cocoa/Cocoa.h>
#import <CoreServices/CoreServices.h>
#include <string.h>

// Finder and `open` hand a launching app its documents in an open-documents
// Apple Event, not as arguments. SDL finishes launching NSApp before it becomes
// the app delegate, so the event can reach AppKit before SDL can turn it into a
// drop. These handlers note the launch event and its files on their way to
// AppKit's own handlers, which still run as before.

static AEEventHandlerUPP gopdfAppKitOpenApplication;
static SRefCon gopdfAppKitOpenApplicationRefcon;
static AEEventHandlerUPP gopdfAppKitOpenDocuments;
static SRefCon gopdfAppKitOpenDocumentsRefcon;

static NSMutableArray<NSString *> *gopdfLaunchPaths;
static BOOL gopdfLaunchEventSeen;
static BOOL gopdfLaunchFinished;
static BOOL gopdfLaunchTaken;

static OSErr gopdfForwardAppleEvent(AEEventHandlerUPP handler, SRefCon refcon, const AppleEvent *event, AppleEvent *reply) {
    if (handler == NULL) {
        return noErr;
    }
    return InvokeAEEventHandlerUPP(event, reply, refcon, handler);
}

static void gopdfRecordLaunchDocuments(const AppleEvent *event) {
    AEDesc copy;
    if (AEDuplicateDesc(event, &copy) != noErr) {
        return;
    }
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

static OSErr gopdfHandleOpenApplication(const AppleEvent *event, AppleEvent *reply, SRefCon refcon) {
    (void)refcon;
    gopdfLaunchEventSeen = YES;
    return gopdfForwardAppleEvent(gopdfAppKitOpenApplication, gopdfAppKitOpenApplicationRefcon, event, reply);
}

static OSErr gopdfHandleOpenDocuments(const AppleEvent *event, AppleEvent *reply, SRefCon refcon) {
    (void)refcon;
    if (!gopdfLaunchTaken) {
        @autoreleasepool {
            gopdfRecordLaunchDocuments(event);
        }
        gopdfLaunchEventSeen = YES;
    }
    return gopdfForwardAppleEvent(gopdfAppKitOpenDocuments, gopdfAppKitOpenDocumentsRefcon, event, reply);
}

static void gopdfWrapAppleEventHandler(AEEventID eventID, AEEventHandlerProcPtr wrapper, AEEventHandlerUPP *appKitHandler, SRefCon *appKitRefcon) {
    if (AEGetEventHandler(kCoreEventClass, eventID, appKitHandler, appKitRefcon, false) != noErr) {
        *appKitHandler = NULL;
        *appKitRefcon = NULL;
    }
    AEInstallEventHandler(kCoreEventClass, eventID, NewAEEventHandlerUPP(wrapper), NULL, false);
}

// gopdfWatchLaunchEvents must run before SDL creates NSApp: AppKit installs its
// handlers just before it announces it will finish launching, and dispatches
// the launch event after.
void gopdfWatchLaunchEvents(void) {
    static BOOL watching;
    if (watching) {
        return;
    }
    watching = YES;
    if (NSApp != nil) {
        gopdfLaunchFinished = YES;
        return;
    }

    @autoreleasepool {
        gopdfLaunchPaths = [[NSMutableArray alloc] init];
        NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
        [center addObserverForName:NSApplicationWillFinishLaunchingNotification
                            object:nil
                             queue:nil
                        usingBlock:^(NSNotification *notification) {
            (void)notification;
            gopdfWrapAppleEventHandler(kAEOpenApplication, gopdfHandleOpenApplication,
                                       &gopdfAppKitOpenApplication, &gopdfAppKitOpenApplicationRefcon);
            gopdfWrapAppleEventHandler(kAEOpenDocuments, gopdfHandleOpenDocuments,
                                       &gopdfAppKitOpenDocuments, &gopdfAppKitOpenDocumentsRefcon);
        }];
        [center addObserverForName:NSApplicationDidFinishLaunchingNotification
                            object:nil
                             queue:nil
                        usingBlock:^(NSNotification *notification) {
            (void)notification;
            gopdfLaunchFinished = YES;
        }];
    }
}

// gopdfLaunchSettled reports whether the launch event has been handled, so its
// documents, if any, are known.
int gopdfLaunchSettled(void) {
    return gopdfLaunchEventSeen || gopdfLaunchFinished;
}

int gopdfLaunchDocumentCount(void) {
    return (int)gopdfLaunchPaths.count;
}

// gopdfLaunchDocument returns a copy of the path, which the caller frees.
char *gopdfLaunchDocument(int index) {
    @autoreleasepool {
        return strdup(gopdfLaunchPaths[index].fileSystemRepresentation);
    }
}

// gopdfFinishLaunchDocuments stops recording; later documents reach the viewer
// as SDL drops.
void gopdfFinishLaunchDocuments(void) {
    gopdfLaunchTaken = YES;
    [gopdfLaunchPaths release];
    gopdfLaunchPaths = nil;
}
