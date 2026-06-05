# TypeScript Go Guaranteed

`typescript-go-guaranteed` is an independently maintained open source fork of the native TypeScript compiler preview. It keeps the upstream project layout intentionally close to `microsoft/typescript-go` while adding runtime guarantee checks and migration tooling.

The npm packages are published under the `@ts-guaranteed` scope:

```sh
bun add --dev @ts-guaranteed/tsgo
bunx tsgo --help
```

The installable package is `@ts-guaranteed/tsgo`. Platform-specific binary packages are installed as optional dependencies with names like `@ts-guaranteed/tsgo-linux-x64`, `@ts-guaranteed/tsgo-darwin-arm64`, and `@ts-guaranteed/tsgo-win32-x64`.

## Runtime Guarantees

Start by scanning an existing project without changing files:

```sh
bunx tsgo guarantee scan .
```

Print the starter config for a project:

```sh
bunx tsgo guarantee init .
```

Preview safe annotations that can be inserted automatically:

```sh
bunx tsgo guarantee scan --strict --fix-dry-run .
```

Apply safe annotations:

```sh
bunx tsgo guarantee scan --strict --fix .
```

Useful ignore examples:

```sh
bunx tsgo guarantee scan --ignore public --ignore "*.generated.js" .
```

## Migrating Projects

1. Install side-by-side with TypeScript:

   ```sh
   bun add --dev @ts-guaranteed/tsgo
   ```

2. Add scripts without replacing your existing `tsc` workflow:

   ```json
   {
     "scripts": {
       "typecheck": "tsc --noEmit",
       "typecheck:tsgo": "tsgo --noEmit",
       "guarantee:scan": "tsgo guarantee scan .",
       "guarantee:strict": "tsgo guarantee scan --strict ."
     }
   }
   ```

3. Run `bun run typecheck:tsgo` in CI as an informational job first.

4. Run `bun run guarantee:scan` and triage findings into modeled runtime contracts, explicit unsafe boundaries, or ignored generated/vendor paths.

5. Promote to `guarantee:strict` only after the project has modeled the reported risks.

For config-based adoption, start with:

```json
{
  "compilerOptions": {
    "runtimeGuarantees": "observe"
  }
}
```

Move to `"runtimeGuarantees": "strict"` once reported risks have been handled.

## Releasing to npm

The repo builds one wrapper package and the platform packages required by that wrapper. The generated `built/npm/publish-order.json` controls publish order so platform packages are published before `@ts-guaranteed/tsgo`.

Build and pack locally:

```sh
bun install --frozen-lockfile
bun run release:build --prerelease alpha.0
```

Dry-run publish locally:

```sh
bun run release:publish:dry-run --tag next
```

Publish locally:

```sh
npm login
bun run release:publish --tag next
```

If your npm account requires one-time passwords:

```sh
bun run release:publish --tag next --otp 123456
```

You can also provide the OTP through the environment:

```sh
NPM_OTP=123456 bun run release:publish --tag next
```

If publishing fails with `E403` and npm says that two-factor authentication or a granular access token with bypass 2FA is required, use one of these paths:

- Local publish: rerun with `--otp <code>` or `NPM_OTP=<code>`.
- GitHub Actions publish: create a granular npm access token with write access for the `@ts-guaranteed/tsgo*` packages and enable bypass 2FA for that token, then store it as the `NPM_TOKEN` secret on the `npm` environment.
- Longer-term GitHub Actions publish: configure npm trusted publishing for the release workflow and remove the long-lived token.

GitHub Actions also includes a manual **Publish npm packages** workflow. Run it with `dry_run: true` first, inspect the uploaded tarballs, then rerun with `dry_run: false` when ready.

## Repository Setup

Before making the repository public:

- Create the npm organization or scope `@ts-guaranteed`.
- Ensure your npm account or organization has permission to publish `@ts-guaranteed/tsgo*`.
- Add a GitHub Actions environment named `npm` and require manual approval for it.
- Add `NPM_TOKEN` as an environment secret for `npm`, unless you switch the workflow to npm trusted publishing.
- Enable Dependabot alerts, secret scanning, push protection, and CodeQL.
- Enable GitHub Discussions only if you want support questions outside issues.

Recommended branch rules for `main`:

- Require pull requests before merging.
- Require one approving review.
- Dismiss stale approvals when new commits are pushed.
- Require conversation resolution before merge.
- Require status checks: `build`, `package`, `extension`, `format`, `lint (ubuntu-latest)`, and at least `test (ubuntu-latest)`.
- Require linear history.
- Block force pushes and deletions.
- Restrict who can bypass rules.
- Use merge queue only after CI duration is understood.

## Upstream Merge Policy

This fork is intentionally shaped to keep upstream merges manageable:

- Keep the Go module path and internal import paths as `github.com/microsoft/typescript-go` unless a full module rename is explicitly planned.
- Keep upstream directory names such as `_packages/native-preview`.
- Keep Hereby task names such as `native-preview:*`.
- Keep VS Code setting and command IDs stable unless the extension is intentionally rebranded.
- Put fork-specific behavior in small, obvious patches around package metadata, release scripts, docs, and runtime guarantee code.

When pulling from upstream:

```sh
git fetch upstream
git merge upstream/main
bun install --frozen-lockfile
bun test
```

Resolve conflicts by preserving upstream structure first, then reapplying fork-specific package and release metadata where needed.

## Status

This remains a preview compiler and is not yet a drop-in replacement for every TypeScript workflow. Keep `tsc` available during migration and compare diagnostics before enforcing `tsgo` in production CI.

## License

This project is licensed under the Apache-2.0 license. See [LICENSE](LICENSE).
