// cgo_darwin.go — macOS note.
// The opus CGO library may produce "malformed LC_DYSYMTAB" linker warnings on
// newer macOS toolchains. These are harmless.
// Suppressed at build time by CGO_LDFLAGS="-Wl,-w" (see Makefile test target).

package codec
