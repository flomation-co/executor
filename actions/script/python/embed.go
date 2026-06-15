// Package script_python embeds CPython 3.12's official WASI build
// inside the executor binary and runs user scripts through wazero
// (pure-Go Wasm runtime, no CGo). See the design notes in the
// planning conversation for why this combination was chosen over
// alternatives like starlark-go (not real Python) or go-python3
// (requires CPython on the host).
package script_python

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// cpythonWasmZstd is the zstd-compressed CPython 3.12 WASI build.
// Compression matters at rest because the raw wasm is 25 MB; the
// zstd form is ~8 MB, which keeps the executor's source tree
// reasonable while still hermetic — no build-time download.
//
// The blob is the official VMware Labs webassembly-language-runtimes
// release for python 3.12.0+20231211-040d5a6. Replacing it means
// also re-running any Python-action tests against the new build.
//
//go:embed assets/cpython-3.12.wasm.zst
var cpythonWasmZstd []byte

// Lazy one-time decompression. Decoding takes ~80ms; doing it at
// init would slow every executor cold-start (including unit tests
// that never touch the Python action). Caching is process-global —
// the decompressed bytes are also feed into wazero's compilation
// cache on first use.
var (
	cpythonWasmOnce sync.Once
	cpythonWasm     []byte
	cpythonWasmErr  error
)

// loadCPythonWasm returns the decompressed CPython WASI bytes,
// decompressing on first call and reusing the same slice on every
// subsequent call. Returns an error only if the embedded blob is
// missing or corrupted — at which point the action surfaces a
// clear "Python runtime asset is corrupt" message instead of a
// confusing wazero parse error.
func loadCPythonWasm() ([]byte, error) {
	cpythonWasmOnce.Do(func() {
		if len(cpythonWasmZstd) == 0 {
			cpythonWasmErr = fmt.Errorf("embedded cpython wasm asset is missing — executor was built without the WASI Python runtime")
			return
		}
		dec, err := zstd.NewReader(bytes.NewReader(cpythonWasmZstd))
		if err != nil {
			cpythonWasmErr = fmt.Errorf("init zstd decoder: %w", err)
			return
		}
		defer dec.Close()
		buf, err := io.ReadAll(dec)
		if err != nil {
			cpythonWasmErr = fmt.Errorf("decompress cpython wasm: %w", err)
			return
		}
		cpythonWasm = buf
	})
	return cpythonWasm, cpythonWasmErr
}
