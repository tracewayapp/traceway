// Package appdir names the binary and the directory the CLI keeps its config
// and state under. The write build sets both to "traceway-write" so its
// credentials never mix with a read install's and its hints name itself.
package appdir

var (
	Name   = "traceway"
	Binary = "traceway"
)
