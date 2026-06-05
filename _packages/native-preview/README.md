# `@ts-guaranteed/tsgo`

`@ts-guaranteed/tsgo` packages the native TypeScript compiler preview with runtime guarantee checks from the `typescript-go-guaranteed` fork.

## Usage

```sh
bun add --dev @ts-guaranteed/tsgo
bunx tsgo --help
```

Run a TypeScript check:

```sh
bunx tsgo --noEmit
```

Scan runtime guarantee risks:

```sh
bunx tsgo guarantee scan .
```

Preview strict-mode migration fixes:

```sh
bunx tsgo guarantee scan --strict --fix-dry-run .
```

## Platform Packages

This package depends on optional platform packages named `@ts-guaranteed/tsgo-<platform>-<arch>`. Bun, npm, pnpm, and Yarn install the matching package for the current system automatically.

## Feedback

Report issues at https://github.com/riley-wv/typescript-go-guaranteed/issues.
