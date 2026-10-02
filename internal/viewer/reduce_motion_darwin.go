//go:build darwin

package viewer

/*
int gopdfReduceMotion(void);
*/
import "C"

// osReducesMotion reports whether Reduce motion is on in macOS's
// accessibility settings.
func osReducesMotion() bool { return C.gopdfReduceMotion() != 0 }
