# Oshimai for VS Code

Run a load-test scenario YAML directly from the editor, without switching to the browser or the CLI.

## Use

1. Open a scenario `.yaml` file.
2. Run **Oshimai: Run This Scenario** (Command Palette, or the play icon in the editor toolbar).
3. Progress and the final result print to the **Oshimai** output channel; a notification links straight to the Live Cockpit for the run.

## Settings

| Setting | Default | Description |
|---|---|---|
| `oshimai.serverUrl` | `http://localhost:8080` | Oshimai server to submit runs to |
| `oshimai.defaultVUs` | `5` | Virtual users for a quick editor-triggered run |
| `oshimai.defaultDurationSeconds` | `30` | Duration for a quick editor-triggered run |

## Package locally

```bash
npm install -g @vscode/vsce
cd tools/vscode-extension
vsce package
```

This produces a `.vsix` you can install via **Extensions: Install from VSIX...** in VS Code.
