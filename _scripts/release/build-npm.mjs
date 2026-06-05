#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";

const { values } = parseArgs({
    options: {
        prerelease: { type: "string" },
        skipTests: { type: "boolean" },
        help: { type: "boolean", short: "h" },
    },
});

if (values.help || !values.prerelease) {
    console.log(`Usage: bun run release:build --prerelease <semver-prerelease> [--skipTests]

Examples:
  bun run release:build --prerelease alpha.0
  bun run release:build --prerelease dev.1 --skipTests`);
    process.exit(values.help ? 0 : 1);
}

function run(command, args) {
    const executable = process.platform === "win32" ? `${command}.cmd` : command;
    const result = spawnSync(executable, args, {
        stdio: "inherit",
        shell: false,
    });
    if (result.status !== 0) {
        process.exit(result.status ?? 1);
    }
}

if (!values.skipTests) {
    run("bunx", ["hereby", "test:api"]);
}

const herebyReleaseArgs = [
    "--forRelease",
    `--setPrerelease=${values.prerelease}`,
];

run("bunx", ["hereby", "native-preview:build-packages", ...herebyReleaseArgs]);
run("bunx", ["hereby", "native-preview:pack-packages", ...herebyReleaseArgs]);

const manifest = path.resolve("built", "npm", "publish-order.json");
if (!existsSync(manifest)) {
    console.error(`Expected publish manifest was not created: ${manifest}`);
    process.exit(1);
}

console.log(`npm packages are ready in ${path.resolve("built", "npm")}`);
console.log(`Publish order: ${manifest}`);
