import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as cp from "child_process";

import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
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
  const versionOK = probeVersion(binary, logChannel);
  if (!versionOK) {
    vscode.window.showErrorMessage(
      "Intent: 'intentc' did not respond to --version. See the 'Intent (Extension)' output panel."
    );
    return;
  }

  const serverOptions: ServerOptions = {
    run: { command: binary, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command: binary, args: ["lsp"], transport: TransportKind.stdio },
  };

  // outputChannel is the LSP client's own log (server stderr + protocol
  // trace). VS Code surfaces it as "Intent" in the Output dropdown.
  const clientLog = vscode.window.createOutputChannel("Intent");
  context.subscriptions.push(clientLog);

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "intent" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.intent"),
    },
    outputChannel: clientLog,
    revealOutputChannelOn: 4, // RevealOutputChannelOn.Never — manual reveal only
  };

  client = new LanguageClient("intent", "Intent", serverOptions, clientOptions);
  client.start().then(
    () => logChannel?.appendLine("Language client started successfully."),
    (err) => {
      logChannel?.appendLine(`Language client failed to start: ${err}`);
      vscode.window.showErrorMessage(`Intent: language server failed to start (${err}).`);
    }
  );
  context.subscriptions.push({ dispose: () => client?.stop() });
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

// resolveBinary finds the intentc executable. Priority:
//   1. The explicit 'intent.binaryPath' setting.
//   2. A login shell's `which intentc` (sees full PATH from .zshrc/.bash_profile).
//   3. Common install locations: $GOPATH/bin, ~/go/bin, /usr/local/bin.
// Returns null only when every path fails.
function resolveBinary(explicit: string, log: vscode.OutputChannel): string | null {
  if (explicit) {
    log.appendLine(`Trying explicit path: ${explicit}`);
    if (fs.existsSync(explicit)) {
      return explicit;
    }
    log.appendLine(`  not found at ${explicit}`);
    return null;
  }

  // Login-shell PATH lookup. macOS GUI apps inherit a stripped PATH; the
  // user's real PATH lives in their shell rc files. Sourcing them via a
  // login shell ('bash -lc' / 'zsh -lc') gives us the same view they
  // get in Terminal.
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

  // Common install locations.
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

// probeVersion runs `intentc --version` to confirm the binary is
// executable and not a stub. Logs the output for diagnostic value.
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
