// Oshimai Record & Replay — background service worker.
//
// Records every outgoing request while recording is active, in HAR-entry shape, so the captured
// session can be exported as a real .har file (importable anywhere) or POSTed straight to
// Oshimai's existing /api/v1/scenarios/import/har endpoint — no new import format to write.

const STORAGE_KEY = 'oshimai_recording_entries';
const STATE_KEY = 'oshimai_recording_active';

async function isRecording() {
  const { [STATE_KEY]: active } = await chrome.storage.local.get(STATE_KEY);
  return !!active;
}

async function appendEntry(entry) {
  const { [STORAGE_KEY]: entries = [] } = await chrome.storage.local.get(STORAGE_KEY);
  entries.push(entry);
  await chrome.storage.local.set({ [STORAGE_KEY]: entries });
}

// Track in-flight requests by requestId so we can attach headers/body captured across the two
// separate webRequest events into one HAR entry.
const pending = new Map();

chrome.webRequest.onBeforeRequest.addListener(
  (details) => {
    if (details.tabId < 0) return; // Ignore extension/background traffic, only capture page requests.
    let postData;
    if (details.requestBody?.raw?.[0]?.bytes) {
      try {
        postData = new TextDecoder('utf-8').decode(new Uint8Array(details.requestBody.raw[0].bytes));
      } catch {
        postData = undefined;
      }
    } else if (details.requestBody?.formData) {
      postData = JSON.stringify(details.requestBody.formData);
    }
    pending.set(details.requestId, {
      method: details.method,
      url: details.url,
      postData,
      startedAt: Date.now(),
    });
  },
  { urls: ['<all_urls>'] },
  ['requestBody']
);

chrome.webRequest.onSendHeaders.addListener(
  (details) => {
    const entry = pending.get(details.requestId);
    if (entry) entry.headers = details.requestHeaders || [];
  },
  { urls: ['<all_urls>'] },
  ['requestHeaders']
);

chrome.webRequest.onCompleted.addListener(async (details) => {
  const captured = pending.get(details.requestId);
  pending.delete(details.requestId);
  if (!captured) return;
  if (!(await isRecording())) return;

  const harEntry = {
    request: {
      method: captured.method,
      url: captured.url,
      headers: (captured.headers || []).map((h) => ({ name: h.name, value: h.value })),
      postData: captured.postData ? { mimeType: 'application/json', text: captured.postData } : undefined,
    },
  };
  await appendEntry(harEntry);
});

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  (async () => {
    if (msg.type === 'START_RECORDING') {
      await chrome.storage.local.set({ [STATE_KEY]: true, [STORAGE_KEY]: [] });
      sendResponse({ ok: true });
    } else if (msg.type === 'STOP_RECORDING') {
      await chrome.storage.local.set({ [STATE_KEY]: false });
      sendResponse({ ok: true });
    } else if (msg.type === 'GET_STATE') {
      const { [STORAGE_KEY]: entries = [] } = await chrome.storage.local.get(STORAGE_KEY);
      sendResponse({ recording: await isRecording(), count: entries.length });
    } else if (msg.type === 'GET_HAR') {
      const { [STORAGE_KEY]: entries = [] } = await chrome.storage.local.get(STORAGE_KEY);
      sendResponse({ log: { version: '1.2', creator: { name: 'Oshimai Record & Replay', version: '1.0.0' }, entries } });
    } else if (msg.type === 'CLEAR') {
      await chrome.storage.local.set({ [STORAGE_KEY]: [] });
      sendResponse({ ok: true });
    }
  })();
  return true; // Keep the message channel open for the async response above.
});
