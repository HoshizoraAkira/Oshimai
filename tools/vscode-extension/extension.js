const vscode = require('vscode');
const https = require('https');
const http = require('http');

function postJSON(baseUrl, path, payload) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, baseUrl);
    const client = url.protocol === 'https:' ? https : http;
    const body = Buffer.from(JSON.stringify(payload));
    const req = client.request(
      url,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Content-Length': body.length },
      },
      (res) => {
        let data = '';
        res.on('data', (chunk) => (data += chunk));
        res.on('end', () => {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            try {
              resolve(JSON.parse(data));
            } catch {
              resolve({});
            }
          } else {
            reject(new Error(`Oshimai server returned ${res.statusCode}: ${data}`));
          }
        });
      }
    );
    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

function getJSON(baseUrl, path) {
  return new Promise((resolve, reject) => {
    const url = new URL(path, baseUrl);
    const client = url.protocol === 'https:' ? https : http;
    client
      .get(url, (res) => {
        let data = '';
        res.on('data', (chunk) => (data += chunk));
        res.on('end', () => {
          try {
            resolve(JSON.parse(data));
          } catch (err) {
            reject(err);
          }
        });
      })
      .on('error', reject);
  });
}

function activate(context) {
  const output = vscode.window.createOutputChannel('Oshimai');

  const runScenario = vscode.commands.registerCommand('oshimai.runScenario', async () => {
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.document.languageId !== 'yaml') {
      vscode.window.showWarningMessage('Oshimai: open a scenario YAML file first.');
      return;
    }

    const config = vscode.workspace.getConfiguration('oshimai');
    const serverUrl = config.get('serverUrl');
    const vus = config.get('defaultVUs');
    const durationSeconds = config.get('defaultDurationSeconds');
    const scenarioYAML = editor.document.getText();

    output.show(true);
    output.appendLine(`Submitting ${editor.document.fileName} to ${serverUrl} (${vus} VUs, ${durationSeconds}s)…`);

    try {
      const created = await postJSON(serverUrl, '/api/v1/runs', {
        scenario_yaml: scenarioYAML,
        load_config: {
          profile: 'flat_vu',
          vus,
          duration: durationSeconds * 1e9,
        },
      });
      const runID = created.id;
      output.appendLine(`Run started: ${runID}`);
      vscode.window.showInformationMessage(`Oshimai run started: ${runID}`, 'Open Cockpit').then((choice) => {
        if (choice === 'Open Cockpit') {
          vscode.env.openExternal(vscode.Uri.parse(`${serverUrl}/?run=${runID}`));
        }
      });

      const poll = setInterval(async () => {
        try {
          const run = await getJSON(serverUrl, `/api/v1/runs/${runID}`);
          if (run.status && run.status !== 'running' && run.status !== 'pending') {
            clearInterval(poll);
            output.appendLine(`Run ${runID} finished with status: ${run.status}`);
            if (run.summary) {
              output.appendLine(
                `  requests=${run.summary.total_requests ?? '?'} error_rate=${run.summary.error_rate ?? '?'}`
              );
            }
            vscode.window.showInformationMessage(`Oshimai run ${runID}: ${run.status}`);
          }
        } catch (err) {
          clearInterval(poll);
          output.appendLine(`Polling error: ${err.message}`);
        }
      }, 2000);
    } catch (err) {
      output.appendLine(`Failed: ${err.message}`);
      vscode.window.showErrorMessage(`Oshimai: ${err.message}`);
    }
  });

  const openDashboard = vscode.commands.registerCommand('oshimai.openDashboard', () => {
    const serverUrl = vscode.workspace.getConfiguration('oshimai').get('serverUrl');
    vscode.env.openExternal(vscode.Uri.parse(serverUrl));
  });

  context.subscriptions.push(runScenario, openDashboard, output);
}

function deactivate() {}

module.exports = { activate, deactivate };
