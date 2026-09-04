const dot = document.getElementById('dot');
const statusText = document.getElementById('statusText');
const countText = document.getElementById('countText');
const startBtn = document.getElementById('startBtn');
const stopBtn = document.getElementById('stopBtn');
const exportBtn = document.getElementById('exportBtn');
const clearBtn = document.getElementById('clearBtn');
const sendBtn = document.getElementById('sendBtn');
const serverUrlInput = document.getElementById('serverUrl');
const message = document.getElementById('message');

const SERVER_URL_KEY = 'oshimai_server_url';

function setMessage(text) {
  message.textContent = text || '';
}

function render(state) {
  const recording = !!state.recording;
  const count = state.count || 0;
  dot.classList.toggle('active', recording);
  statusText.textContent = recording ? 'Recording…' : 'Idle';
  countText.textContent = `${count} req`;
  startBtn.style.display = recording ? 'none' : 'block';
  stopBtn.style.display = recording ? 'block' : 'none';
  const hasData = count > 0;
  exportBtn.disabled = !hasData;
  clearBtn.disabled = !hasData || recording;
  sendBtn.disabled = !hasData || recording;
}

async function refresh() {
  const state = await chrome.runtime.sendMessage({ type: 'GET_STATE' });
  render(state);
}

function downloadHar(harLog) {
  const blob = new Blob([JSON.stringify(harLog, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  chrome.downloads.download({
    url,
    filename: `oshimai-recording-${Date.now()}.har`,
    saveAs: true,
  });
}

startBtn.addEventListener('click', async () => {
  await chrome.runtime.sendMessage({ type: 'START_RECORDING' });
  setMessage('');
  refresh();
});

stopBtn.addEventListener('click', async () => {
  await chrome.runtime.sendMessage({ type: 'STOP_RECORDING' });
  refresh();
});

clearBtn.addEventListener('click', async () => {
  await chrome.runtime.sendMessage({ type: 'CLEAR' });
  setMessage('Cleared.');
  refresh();
});

exportBtn.addEventListener('click', async () => {
  const { log } = await chrome.runtime.sendMessage({ type: 'GET_HAR' });
  downloadHar(log);
});

sendBtn.addEventListener('click', async () => {
  const serverUrl = serverUrlInput.value.trim().replace(/\/$/, '');
  if (!serverUrl) {
    setMessage('Enter your Oshimai server URL first.');
    return;
  }
  await chrome.storage.local.set({ [SERVER_URL_KEY]: serverUrl });
  const { log } = await chrome.runtime.sendMessage({ type: 'GET_HAR' });
  setMessage('Sending…');
  try {
    const resp = await fetch(`${serverUrl}/api/v1/scenarios/import/har`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ har: JSON.stringify(log) }),
    });
    if (!resp.ok) {
      setMessage(`Server responded ${resp.status}.`);
      return;
    }
    setMessage('Sent — scenario generated in Oshimai.');
  } catch (err) {
    setMessage(`Could not reach server: ${err.message}`);
  }
});

(async () => {
  const { [SERVER_URL_KEY]: savedUrl } = await chrome.storage.local.get(SERVER_URL_KEY);
  if (savedUrl) serverUrlInput.value = savedUrl;
  refresh();
})();
