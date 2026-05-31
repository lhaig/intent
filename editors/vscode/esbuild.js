// Bundle the extension into a single CommonJS file. Externalises
// `vscode` (the VS Code API is provided at runtime by the host) and
// inlines every npm dependency — including vscode-languageclient.
//
// Result: out/extension.js is one ~50KB file, the .vsix carries no
// node_modules tree, activation is fast.

const esbuild = require("esbuild");
const path = require("path");

const isWatch = process.argv.includes("--watch");
const isProduction = process.argv.includes("--production");

const config = {
  entryPoints: [path.resolve(__dirname, "src/extension.ts")],
  bundle: true,
  outfile: path.resolve(__dirname, "out/extension.js"),
  format: "cjs",
  platform: "node",
  target: "node18",
  external: ["vscode"],
  sourcemap: !isProduction,
  minify: isProduction,
  logLevel: "info",
};

(async () => {
  if (isWatch) {
    const ctx = await esbuild.context(config);
    await ctx.watch();
    console.log("esbuild watching for changes...");
  } else {
    await esbuild.build(config);
  }
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
