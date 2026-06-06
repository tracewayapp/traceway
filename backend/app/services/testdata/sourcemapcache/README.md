# sourcemapcache fixtures

Copied from getsentry/symbolic, `symbolic-testutils/fixtures/sourcemapcache`, at commit `925230e878ea25f6a0af88171fce93b6272cd30d`.

The expected resolutions asserted in `sourcemap_resolver_symbolic_test.go` are transcribed from `symbolic-sourcemapcache/tests/integration.rs` in that repo, converted from symbolic's 0-based line/column convention to the 1-based browser convention our resolver uses.

Do not edit these files by hand: they are the fixed reference point that lets us detect when our symbolicator output drifts. The committed baseline of our resolver's behavior against them lives in `../symbolic_parity_results.txt` (regenerate with `go test ./app/services/ -run TestResolveStackTraceSymbolicParity -record-symbolic`).

## Null sources (`nofiles` fixture)

Source maps can carry `"sources": [null, ...]`: mappings and inlined `sourcesContent` are present but the original file names are not (seen with older uglify/Raven-era pipelines). Symbolic resolves these to a nameless file; our resolver emits the placeholder `<unknown>` as the file name (e.g. `<unknown>:3:9`) and still runs function-name extraction against the inlined source content. The parity test encodes symbolic's "no file name" as `<unknown>` to match that convention.

Caveat: go-sourcemap's `SourceContent("")` returns the content of the first empty entry in `sources`, so with multiple null sources the content used for name extraction can belong to the wrong file. Acceptable for the extraction heuristic, but the lookup is ambiguous by nature.
