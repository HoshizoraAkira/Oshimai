import { useState, useEffect, useRef } from 'react';
import { LogEntry } from '../types/models';
import { systemService } from '../services/systemService';
import { streamService } from '../services/streamService';

export function useSystemLogs(isCollapsed = false) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filterLevel, setFilterLevel] = useState('ALL');
  const [isConnected, setIsConnected] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll when logs change
  useEffect(() => {
    if (autoScroll && !isCollapsed && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll, isCollapsed]);

  // Stream system logs via SSE
  useEffect(() => {
    let pollingInterval: any = null;

    const subscription = streamService.subscribeSystemLogsStream(
      (entry: LogEntry) => {
        setIsConnected(true);
        setLogs(prev => {
          const next = [...prev, entry];
          return next.length > 500 ? next.slice(next.length - 500) : next;
        });
      },
      () => {
        setIsConnected(false);
      }
    );

    // Initial snapshot fetch
    systemService.getSystemLogs()
      .then(initialLogs => {
        if (Array.isArray(initialLogs) && initialLogs.length > 0) {
          setLogs(initialLogs);
          setIsConnected(true);
        }
      })
      .catch(() => {
        // Fallback polling if SSE fails
        pollingInterval = setInterval(async () => {
          try {
            const data = await systemService.getSystemLogs();
            if (Array.isArray(data)) {
              setLogs(data);
              setIsConnected(true);
            }
          } catch {
            setIsConnected(false);
          }
        }, 4000);
      });

    return () => {
      subscription.close();
      if (pollingInterval) clearInterval(pollingInterval);
    };
  }, []);

  const clearLogs = async () => {
    try {
      await systemService.clearSystemLogs();
      setLogs([]);
    } catch {
      // ignore
    }
  };

  const filteredLogs = logs.filter(log => {
    if (filterLevel === 'ALL') return true;
    return (log.level || 'INFO').toUpperCase() === filterLevel;
  });

  return {
    logs: filteredLogs,
    rawLogsCount: logs.length,
    autoScroll,
    setAutoScroll,
    filterLevel,
    setFilterLevel,
    isConnected,
    clearLogs,
    logContainerRef,
  };
}
