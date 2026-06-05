# TypeScript Go Guaranteed

This extension runs the native TypeScript language service from the `typescript-go-guaranteed` fork.

## Usage

1. Install the extension.
2. Install `@ts-guaranteed/tsgo` in the workspace, or use the bundled binary included with the extension package.
3. Open a TypeScript or JavaScript file.
4. Run `TypeScript Native Preview: Enable (Experimental)`.

The extension currently keeps the upstream `typescript.native-preview.*` setting and command IDs to reduce upstream merge friction.

## Configuration

```jsonc
{
    "js/ts.experimental.useTsgo": true,
    "typescript.native-preview.tsdk": "node_modules/@ts-guaranteed/tsgo"
}
```

## Feedback

Report issues at https://github.com/riley-wv/typescript-go-guaranteed/issues.
