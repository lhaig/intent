import * as path from "path";
import * as fs from "fs";
import * as cp from "child_process";

import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export function activate(context: vscode.ExtensionContext): void {
  const config = vscode.workspace.getConfiguration("intent");
  const explicit = config.get<string>("binaryPath", "").trim();

  const binary = resolveBinary(explicit);
  if (!binary) {
    vscode.window.showErrorMessage(
      "Intent: could not find 'intentc' on PATH. Install it (https://github.com/lhaig/intent) " +
        "or set 'intent.binaryPath' to point at the binary."
    );
    return;
  }

  const serverOptions: ServerOptions = {
    run: { command: binary, args: ["lsp"], transport: TransportKind.stdio },
    debug: { command: binary, args: ["lsp"], transport: TransportKind.stdio },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "intent" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.intent"),
    },
  };

  client = new LanguageClient("intent", "Intent", serverOptions, clientOptions);
  client.start();
  context.subscriptions.push({ dispose: () => client?.stop() });
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

// resolveBinary finds the intentc executable. Priority:
//   1. The explicit 'intent.binaryPath' setting (if non-empty).
//   2. The PATH lookup ('which intentc' / 'where intentc').
// Returns null when nothing resolves so the caller can show a clear error.
function resolveBinary(explicit: string): string | null {
  if (explicit) {
    if (fs.existsSync(explicit)) {
      return explicit;
    }
    return null;
  }
  // PATH lookup via shell. Cross-platform: 'where' on Windows, 'which' elsewhere.
  const lookup = process.platform === "win32" ? "where" : "which";
  try {
    const out = cp.execFileSync(lookup, ["intentc"], { encoding: "utf8" });
    const first = out.split(/\r?\n/)[0].trim();
    return first || null;
  } catch {
    return null;
  }
}

// path is imported but only used here for future expansion (e.g., resolving
// workspace-relative binary paths). The reference avoids an unused-import
// warning in strict mode.
void path;
