#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";

const { values } = parseArgs({
    options: {
        tag: { type: "string" },
        otp: { type: "string" },
        dryRun: { type: "boolean" },
        provenance: { type: "boolean" },
        help: { type: "boolean", short: "h" },
    },
});

if (values.help) {
    console.log(`Usage: bun run release:publish [--tag <dist-tag>] [--dryRun] [--otp <code>] [--provenance]

Examples:
  bun run release:publish --dryRun --tag next
  bun run release:publish --tag latest --otp 123456
  NPM_OTP=123456 bun run release:publish --tag latest
  bun run release:publish --tag next --provenance`);
    process.exit(0);
}

const publishDir = path.resolve("built", "npm");
const manifestPath = path.join(publishDir, "publish-order.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const otp = values.otp ?? process.env.NPM_OTP;

function run(command, args) {
    const executable = process.platform === "win32" ? `${command}.cmd` : command;
    console.log(`> ${command} ${args.join(" ")}`);
    const result = spawnSync(executable, args, {
        stdio: "inherit",
        shell: false,
    });
    if (result.status !== 0) {
        process.exit(result.status ?? 1);
    }
}

for (const entry of manifest) {
    const tarball = path.join(publishDir, entry.filename);
    const tag = values.tag ?? entry.tag ?? "latest";
    const args = [
        "publish",
        tarball,
        "--access",
        "public",
        "--tag",
        tag,
    ];

    if (values.dryRun) {
        args.push("--dry-run");
    }
    if (values.provenance) {
        args.push("--provenance");
    }
    if (otp) {
        args.push("--otp", otp);
    }

    run("npm", args);
}
