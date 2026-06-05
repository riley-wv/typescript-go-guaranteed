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

func TestGuaranteeScanFixWritesModeledRuntimeClauses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "app.ts")
	input := `"use runtime guaranteed";
export async function load(url: string): string {
  while (url.length) {
    break;
  }
  const raw = await fetch(url);
  return JSON.parse(await raw.text());
}
`
	assert.NilError(t, os.WriteFile(file, []byte(input), 0o644))

	exitCode := runGuaranteeScan([]string{"--strict", "--fix", root})

	assert.Equal(t, exitCode, 0)
	actual, err := os.ReadFile(file)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), `"use runtime guaranteed";
export async function load(url: string): string throws [Error] effects [network] validates [unknown] {
  /** @runtime-bounded */
  while (url.length) {
    break;
  }
  const raw = await fetch(url);
  return JSON.parse(await raw.text());
}
`)
}

func TestGuaranteeScanFixWritesClausesAfterParameterList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "app.ts")
	input := `export async function load(url: string) {
  await fetch(url);
}
export const capture = async (message: string, props?: Record<string, unknown>) => {
  await import(message);
};
export function Component() {
  useEffect(() => {
    setTimeout(() => {}, 0);
  }, []);
}
`
	assert.NilError(t, os.WriteFile(file, []byte(input), 0o644))

	exitCode := runGuaranteeScan([]string{"--fix", root})

	assert.Equal(t, exitCode, 0)
	actual, err := os.ReadFile(file)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), `export async function load(url: string) effects [network] {
  await fetch(url);
}
export const capture = async (message: string, props?: Record<string, unknown>) effects [module] => {
  await import(message);
};
export function Component() {
  useEffect(() effects [time] => {
    setTimeout(() => {}, 0);
  }, []);
}
`)
}

func TestGuaranteeScanFixDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "app.ts")
	input := `"use runtime guaranteed";
export function load(url: string): string {
  return JSON.parse(url);
}
`
	assert.NilError(t, os.WriteFile(file, []byte(input), 0o644))

	exitCode := runGuaranteeScan([]string{"--strict", "--fix-dry-run", root})

	assert.Equal(t, exitCode, 1)
	actual, err := os.ReadFile(file)
	assert.NilError(t, err)
	assert.Equal(t, string(actual), input)
}
