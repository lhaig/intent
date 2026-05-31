import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as cp from "child_process";

import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  StreamInfo,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;
let logChannel: vscode.OutputChannel | undefined;

export function activate(context: vscode.ExtensionContext): void {
  logChannel = vscode.window.createOutputChannel("Intent (Extension)");
  context.subscriptions.push(logChannel);

  const config = vscode.workspace.getConfiguration("intent");
  const explicit = config.get<string>("binaryPath", "").trim();

  const binary = resolveBinary(explicit, logChannel);
  if (!binary) {
    const msg =
      "Intent: could not find 'intentc' on PATH. Set 'intent.binaryPath' " +
      "to the absolute path of the binary, or install intentc on PATH. " +
      "See the 'Intent (Extension)' output panel for details.";
    vscode.window.showErrorMessage(msg);
    logChannel.appendLine(msg);
    return;
  }

  logChannel.appendLine(`Resolved intentc: ${binary}`);
  if (!probeVersion(binary, logChannel)) {
    vscode.window.showErrorMessage(
      "Intent: 'intentc' did not respond to --version. See the 'Intent (Extension)' output panel."
    );
    return;
  }
  if (!probeLspSubcommand(binary, logChannel)) {
    vscode.window.showErrorMessage(
      "Intent: this 'intentc' binary does not support the 'lsp' subcommand — it's from an older build. " +
        "Rebuild with `make install` from /Users/lance.haig/dev/ai/exp/intent. " +
        "See the 'Intent (Extension)' output panel for details."
    );
    return;
  }

  // Spawn the server ourselves so we can capture stderr. vscode-languageclient
  // doesn't surface child-process stderr cleanly by default; routing it
  // through our own log channel makes server crashes visible.
  const serverOptions: ServerOptions = () =>
    new Promise<StreamInfo>((resolve, reject) => {
      const child = cp.spawn(binary, ["lsp"], {
        env: { ...process.env, PATH: process.env.PATH },
      });

      child.on("error", (err) => {
        logChannel?.appendLine(`spawn error: ${err.stack || err}`);
        reject(err);
      });
      child.on("exit", (code, signal) => {
        logChannel?.appendLine(`server exited: code=${code} signal=${signal}`);
      });
      child.stderr.on("data", (chunk: Buffer) => {
        logChannel?.appendLine(`server stderr: ${chunk.toString("utf8").trimEnd()}`);
      });

      logChannel?.appendLine(`spawned intentc lsp (pid=${child.pid})`);
      resolve({ reader: child.stdout, writer: child.stdin });
    });

  const clientLog = vscode.window.createOutputChannel("Intent");
  context.subscriptions.push(clientLog);

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "intent" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.intent"),
    },
    outputChannel: clientLog,
    revealOutputChannelOn: 4,
  };

  client = new LanguageClient("intent", "Intent", serverOptions, clientOptions);
  client.start().then(
    () => logChannel?.appendLine("Language client started successfully."),
    (err) => {
      logChannel?.appendLine(`Language client failed to start: ${err.stack || err}`);
      vscode.window.showErrorMessage(`Intent: language server failed to start (${err}).`);
    }
  );
  context.subscriptions.push({ dispose: () => client?.stop() });
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

function resolveBinary(explicit: string, log: vscode.OutputChannel): string | null {
  if (explicit) {
    log.appendLine(`Trying explicit path: ${explicit}`);
    if (fs.existsSync(explicit)) {
      return explicit;
    }
    log.appendLine(`  not found at ${explicit}`);
    return null;
  }

  const shell = process.env.SHELL || "/bin/bash";
  log.appendLine(`Trying login-shell PATH lookup via ${shell}`);
  try {
    const out = cp
      .execSync(`${shell} -lc 'which intentc'`, { encoding: "utf8", timeout: 5000 })
      .trim()
      .split(/\r?\n/)[0]
      .trim();
    if (out && fs.existsSync(out)) {
      log.appendLine(`  found: ${out}`);
      return out;
    }
    log.appendLine(`  login shell returned: ${out || "(empty)"}`);
  } catch (e) {
    log.appendLine(`  login shell lookup failed: ${e}`);
  }

  const home = os.homedir();
  const candidates = [
    process.env.GOPATH ? path.join(process.env.GOPATH, "bin", "intentc") : null,
    path.join(home, "go", "bin", "intentc"),
    "/usr/local/bin/intentc",
    "/opt/homebrew/bin/intentc",
  ].filter((p): p is string => p !== null);

  for (const c of candidates) {
    log.appendLine(`Trying fallback location: ${c}`);
    if (fs.existsSync(c)) {
      log.appendLine(`  found: ${c}`);
      return c;
    }
  }

  log.appendLine("No intentc binary found.");
  return null;
}

function probeVersion(binary: string, log: vscode.OutputChannel): boolean {
  try {
    const out = cp.execFileSync(binary, ["--version"], { encoding: "utf8", timeout: 5000 });
    log.appendLine(`intentc --version: ${out.trim()}`);
    return true;
  } catch (e) {
    log.appendLine(`intentc --version failed: ${e}`);
    return false;
  }
}

// probeLspSubcommand checks that the binary actually has `lsp` in its help.
// Catches the case where an older intentc is on PATH that responds to
// --version but exits immediately on `lsp` (no such subcommand).
function probeLspSubcommand(binary: string, log: vscode.OutputChannel): boolean {
  try {
    const out = cp.execFileSync(binary, ["help"], { encoding: "utf8", timeout: 5000 });
    if (out.includes("intentc lsp")) {
      log.appendLine("Binary supports 'intentc lsp' subcommand.");
      return true;
    }
    log.appendLine(`Binary's help output does not mention 'lsp': ${out.slice(0, 200)}...`);
    return false;
  } catch (e) {
    log.appendLine(`'intentc help' failed: ${e}`);
    return false;
  }
}
