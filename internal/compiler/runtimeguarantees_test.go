package compiler

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnostics"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"gotest.tools/v3/assert"
)

func TestRuntimeGuaranteesObserveReportsSuggestions(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`type User = { id: string };
export async function load(id: User): Promise<User> {
  const raw = await fetch("/users/" + id);
  return JSON.parse(await raw.text()) as User;
}`, false)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeObserve}

	diags := getRuntimeGuaranteeDiagnostics(file, options, true /*suggestions*/)

	assert.Assert(t, len(diags) > 0)
	for _, diag := range diags {
		assert.Equal(t, diag.Category(), diagnostics.CategorySuggestion)
	}
}

func TestRuntimeGuaranteesStrictReportsErrors(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`type User = { id: string };
export async function load(id: User): Promise<User> {
  const raw = await fetch("/users/" + id);
  return JSON.parse(await raw.text()) as User;
}`, false)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeStrict}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Assert(t, len(diags) > 0)
	for _, diag := range diags {
		assert.Equal(t, diag.Category(), diagnostics.CategoryError)
	}
}

func TestRuntimeGuaranteedDirectiveEnforcesFile(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
export function parse(input: string): string {
  return JSON.parse(input);
}`, false)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Assert(t, len(diags) > 0)
	for _, diag := range diags {
		assert.Equal(t, diag.Category(), diagnostics.CategoryError)
	}
}

func TestRuntimeGuaranteeAnnotationsModelKnownRisks(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
/**
 * @runtime-effects network
 * @runtime-throws
 * @runtime-validates
 */
export async function load(url: string): string {
  await fetch(url);
  return JSON.parse("\"ok\"");
}`, false)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Equal(t, len(diags), 0)
}

func TestRuntimeUnsafeAnnotationSuppressesSubtree(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
/** @runtime-unsafe */
export function legacy(input) {
  return JSON.parse(input);
}`, false)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Equal(t, len(diags), 0)
}

func TestRuntimeGuaranteeSyntaxModelsKnownRisks(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
export async function load(url: string): string throws [NetworkError, SyntaxError] effects [network] validates [string] total {
  while (true) {
    break;
  }
  const raw = await fetch(url);
  return JSON.parse(await raw.text());
}`, false)
	assert.Equal(t, len(file.Diagnostics()), 0)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Equal(t, len(diags), 0)
}

func TestRuntimeGuaranteeSyntaxRequiresDeclaredEffects(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
export async function load(url: string): string throws [NetworkError] {
  await fetch(url);
  return "ok";
}`, false)
	assert.Equal(t, len(file.Diagnostics()), 0)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Assert(t, len(diags) > 0)
}

func TestRuntimeGuaranteeSyntaxEmptyThrowsDoesNotModelAwait(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
export async function load(url: string): string throws [] effects [network] {
  await fetch(url);
  return "ok";
}`, false)
	assert.Equal(t, len(file.Diagnostics()), 0)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Assert(t, len(diags) > 0)
}

func TestRuntimeGuaranteeSyntaxParsesOnArrows(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
export const load = (url: string): string throws [NetworkError] effects [network] => "ok";`, false)
	assert.Equal(t, len(file.Diagnostics()), 0)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Equal(t, len(diags), 0)
}

func TestRuntimeGuaranteeSyntaxParsesOnSignatures(t *testing.T) {
	t.Parallel()
	file := parsetestutil.ParseTypeScript(`"use runtime guaranteed";
interface Api {
  load(url: string): string throws [NetworkError] effects [network];
}
type Loader = (url: string) => string throws [NetworkError] effects [network];`, false)
	assert.Equal(t, len(file.Diagnostics()), 0)
	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeOff}

	diags := getRuntimeGuaranteeDiagnostics(file, options, false /*suggestions*/)

	assert.Equal(t, len(diags), 0)
}
