package tstransforms_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/printer"
	"github.com/microsoft/typescript-go/internal/testutil/emittestutil"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"github.com/microsoft/typescript-go/internal/transformers"
	"github.com/microsoft/typescript-go/internal/transformers/tstransforms"
)

func TestRuntimeGuaranteesModes(t *testing.T) {
	t.Parallel()
	data := []struct {
		title  string
		mode   core.RuntimeGuaranteesMode
		input  string
		output string
	}{
		{
			title:  "off",
			mode:   core.RuntimeGuaranteesModeOff,
			input:  "export function f(x: string): number { return x.length; }",
			output: `export function f(x) { return x.length; }`,
		},
		{
			title:  "observe",
			mode:   core.RuntimeGuaranteesModeObserve,
			input:  "export function f(x: string): number { return x.length; }",
			output: `export function f(x) { return x.length; }`,
		},
		{
			title: "boundary",
			mode:  core.RuntimeGuaranteesModeBoundary,
			input: "function local(x: string): number { return x.length; }\nexport function f(x: string): number { return x.length; }\nexport const g = (x: string): number => x.length;",
			output: `var __tsg = (this && this.__tsg) || function (value, tag, label, optional) {
    if (optional && value === void 0) return value;
    var actual = value === null ? "null" : typeof value;
    var ok = tag === "object" ? value !== null && (actual === "object" || actual === "function") : actual === tag;
    if (!ok) throw new TypeError("Runtime guarantee failed for " + label + ": expected " + tag + ", got " + actual + ".");
    return value;
};
function local(x) { return x.length; }
export function f(x) { __tsg(x, "string", "x", false); return __tsg(x.length, "number", "f return", false); }
export const g = (x) => {
    __tsg(x, "string", "x", false);
    return __tsg(x.length, "number", "function return", false);
};`,
		},
		{
			title: "all",
			mode:  core.RuntimeGuaranteesModeAll,
			input: "const f = (x: string): number => x.length;",
			output: `var __tsg = (this && this.__tsg) || function (value, tag, label, optional) {
    if (optional && value === void 0) return value;
    var actual = value === null ? "null" : typeof value;
    var ok = tag === "object" ? value !== null && (actual === "object" || actual === "function") : actual === tag;
    if (!ok) throw new TypeError("Runtime guarantee failed for " + label + ": expected " + tag + ", got " + actual + ".");
    return value;
};
const f = (x) => {
    __tsg(x, "string", "x", false);
    return __tsg(x.length, "number", "function return", false);
};`,
		},
		{
			title: "strict",
			mode:  core.RuntimeGuaranteesModeStrict,
			input: "const f = (x: string): number => x.length;",
			output: `var __tsg = (this && this.__tsg) || function (value, tag, label, optional) {
    if (optional && value === void 0) return value;
    var actual = value === null ? "null" : typeof value;
    var ok = tag === "object" ? value !== null && (actual === "object" || actual === "function") : actual === tag;
    if (!ok) throw new TypeError("Runtime guarantee failed for " + label + ": expected " + tag + ", got " + actual + ".");
    return value;
};
const f = (x) => {
    __tsg(x, "string", "x", false);
    return __tsg(x.length, "number", "function return", false);
};`,
		},
	}

	for _, rec := range data {
		t.Run(rec.title, func(t *testing.T) {
			t.Parallel()
			file := parsetestutil.ParseTypeScript(rec.input, false)
			parsetestutil.CheckDiagnostics(t, file)
			compilerOptions := &core.CompilerOptions{RuntimeGuarantees: rec.mode}
			emitContext := printer.NewEmitContext()
			if guaranteeTx := tstransforms.NewRuntimeGuaranteesTransformer(&transformers.TransformOptions{CompilerOptions: compilerOptions, Context: emitContext}); guaranteeTx != nil {
				file = guaranteeTx.TransformSourceFile(file)
			}
			file = tstransforms.NewTypeEraserTransformer(&transformers.TransformOptions{CompilerOptions: compilerOptions, Context: emitContext}).TransformSourceFile(file)
			emittestutil.CheckEmit(t, emitContext, file, rec.output)
		})
	}
}
