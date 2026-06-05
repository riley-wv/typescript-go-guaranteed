# Contributing

`typescript-go-guaranteed` is an independently maintained fork that intentionally stays close to upstream `microsoft/typescript-go`.

## Upstream Compatibility

Keep upstream merges easy:

- Do not rename the Go module or internal Go import paths without a dedicated migration plan.
- Do not rename `_packages/native-preview`, Hereby `native-preview:*` tasks, or VS Code `typescript.native-preview.*` command and setting IDs just for branding.
- Prefer small fork-specific patches around runtime guarantee behavior, package metadata, release automation, and docs.
- Avoid broad formatting-only changes.

## Setup

```sh
git clone --recurse-submodules https://github.com/riley-wv/typescript-go-guaranteed.git
cd typescript-go-guaranteed
bun install --frozen-lockfile
```

If you cloned without submodules:

```sh
git submodule update --init --recursive
```

Requirements:

- Go 1.26 or newer.
- Node.js compatible with the root `package.json` engines field.
- Bun 1.3.13 or newer.
- npm only when publishing packages to the npm registry.

## Common Tasks

```sh
bun run build
bun test
bunx hereby lint
bunx hereby check:format
bunx hereby native-preview:pack-packages
```

Run the local binary after a build:

```sh
built/local/tsgo --help
```

## Runtime Guarantee Work

Use the onboarding scanner while developing guarantee-related changes:

```sh
built/local/tsgo guarantee scan .
built/local/tsgo guarantee scan --strict --fix-dry-run .
```

## Pull Requests

Pull requests should include:

- A focused description of the change.
- Tests or an explanation of why tests are not practical.
- Any known upstream merge implications.
- Disclosure when AI tools authored substantial parts of the patch.

Bulk automated contributions are not accepted. A human contributor must choose the change, review the patch, and respond to review feedback.
