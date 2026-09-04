/**
 * Stream Service: encapsulates EventSource Server-Sent Events (SSE) connections.
 */

export interface StreamSubscription {
  close: () => void;
}

export const streamService = {
  subscribeRunStream(
    runId: string,
    onMessage: (data: any) => void,
    onError?: (err: any) => void
  ): StreamSubscription {
    const es = new EventSource(`/api/v1/runs/${runId}/stream`);

    es.onmessage = (event: MessageEvent) => {
      try {
        const parsed = JSON.parse(event.data);
        onMessage(parsed);
      } catch (err) {
        console.error('SSE JSON parse error in run stream:', err);
      }
    };

    es.onerror = (err) => {
      if (onError) onError(err);
    };

    return {
      close: () => es.close(),
    };
  },

  subscribePublicRunStream(
    token: string,
    onMessage: (data: any) => void,
    onError?: (err: any) => void
  ): StreamSubscription {
    const es = new EventSource(`/api/v1/public/runs/${token}/stream`);

    es.onmessage = (event: MessageEvent) => {
      try {
        const parsed = JSON.parse(event.data);
        onMessage(parsed);
      } catch {
        // ignore malformed keep-alive frames
      }
    };

    es.onerror = (err) => {
      if (onError) onError(err);
    };

    return {
      close: () => es.close(),
    };
  },

  subscribeSystemLogsStream(
    onMessage: (data: any) => void,
    onError?: (err: any) => void
  ): StreamSubscription {
    const es = new EventSource('/api/v1/system/logs/stream');

    es.onmessage = (event: MessageEvent) => {
      try {
        const parsed = JSON.parse(event.data);
        onMessage(parsed);
      } catch (err) {
        console.error('SSE parse error in system logs stream:', err);
      }
    };

    es.onerror = (err) => {
      if (onError) onError(err);
    };

    return {
      close: () => es.close(),
    };
  },
};
