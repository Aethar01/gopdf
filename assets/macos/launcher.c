#include <limits.h>
#include <mach-o/dyld.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

int main(int argc, char **argv) {
    (void)argc;

    char executable[PATH_MAX];
    uint32_t size = sizeof(executable);
    if (_NSGetExecutablePath(executable, &size) != 0) {
        fprintf(stderr, "GoPDF: launcher path is too long\n");
        return 1;
    }

    char resolved[PATH_MAX];
    if (realpath(executable, resolved) == NULL) {
        perror("GoPDF: realpath");
        return 1;
    }

    char *last_slash = strrchr(resolved, '/');
    if (last_slash == NULL) {
        fprintf(stderr, "GoPDF: invalid launcher path\n");
        return 1;
    }
    *last_slash = '\0';

    char frameworks[PATH_MAX];
    char binary[PATH_MAX];
    if (snprintf(frameworks, sizeof(frameworks), "%s/../Frameworks", resolved) >= (int)sizeof(frameworks) ||
        snprintf(binary, sizeof(binary), "%s/gopdf-bin", resolved) >= (int)sizeof(binary)) {
        fprintf(stderr, "GoPDF: application path is too long\n");
        return 1;
    }

    if (setenv("DYLD_LIBRARY_PATH", frameworks, 1) != 0) {
        perror("GoPDF: setenv");
        return 1;
    }

    argv[0] = binary;
    execv(binary, argv);

    perror("GoPDF: execv");
    return 1;
}
