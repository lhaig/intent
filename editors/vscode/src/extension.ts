import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as cp from "child_process";

import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  State,
  StreamInfo,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;
let logChannel: vscode.OutputChannel | undefined;
let clientLog: vscode.OutputChannel | undefined;
let statusBar: vscode.StatusBarItem | undefined;
let extensionContext: vscode.ExtensionContext | undefined;

export function activate(context: vscode.ExtensionContext): void {
  extensionContext = context;
  logChannel = vscode.window.createOutputChannel("Intent (Extension)");
  clientLog = vscode.window.createOutputChannel("Intent");
  statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 0);
  statusBar.command = "intent.showOutput";
  context.subscriptions.push(logChannel, clientLog, statusBar);

  // Commands.
  context.subscriptions.push(
    vscode.commands.registerCommand("intent.restartServer", restartServer),
    vscode.commands.registerCommand("intent.showOutput", () => clientLog?.show(true))
  );

  setStatus("starting");
  void startClient();
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

async function startClient(): Promise<void> {
  if (!logChannel || !clientLog || !statusBar || !extensionContext) {
    return;
  }
  const config = vscode.workspace.getConfiguration("intent");
  const explicit = config.get<string>("binaryPath", "").trim();

  const binary = resolveBinary(explicit, logChannel);
  if (!binary) {
    setStatus("error");
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
    setStatus("error");
    vscode.window.showErrorMessage(
      "Intent: 'intentc' did not respond to --version. See the 'Intent (Extension)' output panel."
    );
    return;
  }
  if (!probeLspSubcommand(binary, logChannel)) {
    setStatus("error");
    vscode.window.showErrorMessage(
      "Intent: this 'intentc' binary does not support the 'lsp' subcommand — it's from an older build. " +
        "Rebuild with `make install` from the intent repo, then run 'Intent: Restart Server'."
    );
    return;
  }

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
        setStatus("error");
      });
      child.stderr.on("data", (chunk: Buffer) => {
        logChannel?.appendLine(`server stderr: ${chunk.toString("utf8").trimEnd()}`);
      });
      logChannel?.appendLine(`spawned intentc lsp (pid=${child.pid})`);
      resolve({ reader: child.stdout, writer: child.stdin });
    });

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "intent" }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.intent"),
    },
    outputChannel: clientLog,
    revealOutputChannelOn: 4,
  };

  client = new LanguageClient("intent", "Intent", serverOptions, clientOptions);
  extensionContext.subscriptions.push(
    client.onDidChangeState((e) => {
      switch (e.newState) {
        case State.Running:
          setStatus("running");
          break;
        case State.Starting:
          setStatus("starting");
          break;
        case State.Stopped:
          setStatus("stopped");
          break;
      }
    })
  );

  try {
    await client.start();
    logChannel.appendLine("Language client started successfully.");
  } catch (err) {
    setStatus("error");
    logChannel.appendLine(`Language client failed to start: ${err}`);
    vscode.window.showErrorMessage(`Intent: language server failed to start (${err}).`);
  }
}

async function restartServer(): Promise<void> {
  logChannel?.appendLine("Restart requested via command.");
  setStatus("starting");
  if (client) {
    try {
      await client.stop();
    } catch (err) {
      logChannel?.appendLine(`Stop during restart errored (continuing): ${err}`);
    }
    client = undefined;
  }
  await startClient();
}

// setStatus reflects server state in the bottom-left status bar. Colours
// follow VS Code's themeColor convention so they match the user's theme.
type Status = "starting" | "running" | "stopped" | "error";
function setStatus(status: Status): void {
  if (!statusBar) return;
  switch (status) {
    case "running":
      statusBar.text = "$(check) Intent";
      statusBar.tooltip = "Intent language server running. Click to view logs.";
      statusBar.backgroundColor = undefined;
      break;
    case "starting":
      statusBar.text = "$(sync~spin) Intent";
      statusBar.tooltip = "Intent language server starting…";
      statusBar.backgroundColor = undefined;
      break;
    case "stopped":
      statusBar.text = "$(circle-slash) Intent";
      statusBar.tooltip = "Intent language server stopped. Run 'Intent: Restart Server'.";
      statusBar.backgroundColor = new vscode.ThemeColor("statusBarItem.warningBackground");
      break;
    case "error":
      statusBar.text = "$(error) Intent";
      statusBar.tooltip = "Intent language server failed. Click to view logs.";
      statusBar.backgroundColor = new vscode.ThemeColor("statusBarItem.errorBackground");
      break;
  }
  statusBar.show();
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
