//go:build darwin

package viewer

/*
int ononReduceMotion(void);
*/
import "C"

// osReducesMotion reports whether Reduce motion is on in macOS's
// accessibility settings.
func osReducesMotion() bool { return C.ononReduceMotion() != 0 }
