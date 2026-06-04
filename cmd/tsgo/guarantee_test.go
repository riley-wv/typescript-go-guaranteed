package main

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestRuntimeGuaranteeIgnorePatterns(t *testing.T) {
	t.Parallel()
	var patterns runtimeGuaranteeIgnorePatterns
	assert.NilError(t, patterns.Set("public,*.generated.js"))
	assert.NilError(t, patterns.Set("assets/compiled/**"))

	assert.Assert(t, patterns.match("public/app.js", "app.js"))
	assert.Assert(t, patterns.match("src/client.generated.js", "client.generated.js"))
	assert.Assert(t, patterns.match("assets/compiled/chunk/main.js", "main.js"))
	assert.Assert(t, !patterns.match("src/app.ts", "app.ts"))
}

func TestGuaranteeScanIgnoreSkipsGeneratedDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	assert.NilError(t, os.Mkdir(filepath.Join(root, "public"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "public", "compiled.js"), []byte(`fetch("/compiled");`), 0o644))

	exitCode := runGuaranteeScan([]string{"--strict", "--ignore", "public", root})

	assert.Equal(t, exitCode, 0)
}
