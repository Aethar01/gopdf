#include <windows.h>
#include <windowsx.h>
#include <commctrl.h>

// gopdf draws its own title bar buttons, and SDL's hit test can only call
// them part of the window. Windows 11 shows its snap layouts over a window's
// maximize button only when the system is told the pointer is over one, so
// this subclass of SDL's window says so over gopdf's, and turns the presses
// Windows then sends as the frame's own back into the title bar's.

extern int gopdfTitleBarMaximizeAt(int x, int y);
extern void gopdfTitleBarMaximizePress(int down);

static LRESULT CALLBACK gopdfTitleBarProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam,
                                          UINT_PTR id, DWORD_PTR data) {
    (void)data;
    switch (msg) {
    case WM_NCHITTEST: {
        // SDL answers first, which also reveals the buttons.
        LRESULT hit = DefSubclassProc(hwnd, msg, wParam, lParam);
        if (hit == HTCLIENT) {
            POINT p = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            if (ScreenToClient(hwnd, &p) && gopdfTitleBarMaximizeAt(p.x, p.y)) {
                return HTMAXBUTTON;
            }
        }
        return hit;
    }
    case WM_NCLBUTTONDOWN:
    case WM_NCLBUTTONDBLCLK:
        // Left to the system, the press would draw a classic button.
        if (wParam == HTMAXBUTTON) {
            gopdfTitleBarMaximizePress(1);
            return 0;
        }
        break;
    case WM_NCLBUTTONUP:
        if (wParam == HTMAXBUTTON) {
            gopdfTitleBarMaximizePress(0);
            return 0;
        }
        break;
    case WM_NCDESTROY:
        RemoveWindowSubclass(hwnd, gopdfTitleBarProc, id);
        break;
    }
    return DefSubclassProc(hwnd, msg, wParam, lParam);
}

void gopdfSubclassTitleBar(void *hwnd) {
    SetWindowSubclass((HWND)hwnd, gopdfTitleBarProc, 1, 0);
}
