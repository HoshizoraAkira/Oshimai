import { useState, useEffect, useRef } from 'react';
import { Run, RunDiagnostics, TelemetryData } from '../types/models';
import { runsService } from '../services/runsService';
import { streamService } from '../services/streamService';

export function useRunTelemetry(
  currentRunId: string | null,
  onRunFinished?: (run: Run, diagnostics: RunDiagnostics | null, status: string) => void
) {
  const [streamData, setStreamData] = useState<TelemetryData | null>(null);
  const [runDetails, setRunDetails] = useState<Run | null>(null);
  const [diagnostics, setDiagnostics] = useState<RunDiagnostics | null>(null);
  const [status, setStatus] = useState<string>('IDLE');
  const [chaosActive, setChaosActive] = useState<boolean>(false);
  const [faultId, setFaultId] = useState<string>('');
  const [elapsedSec, setElapsedSec] = useState<number>(0);
  const timerRef = useRef<any>(null);

  useEffect(() => {
    if (!currentRunId) {
      setStatus('IDLE');
      setStreamData(null);
      setRunDetails(null);
      setDiagnostics(null);
      return;
    }

    setStatus('RUNNING');
    setElapsedSec(0);
    setRunDetails(null);
    setDiagnostics(null);

    if (timerRef.current) clearInterval(timerRef.current);
    const startMs = Date.now();
    timerRef.current = setInterval(() => {
      setElapsedSec(Math.floor((Date.now() - startMs) / 1000));
    }, 1000);

    const handleRunCompleted = async () => {
      try {
        const run = await runsService.getRun(currentRunId);
        setRunDetails(run);

        const terminalStatus = run.status === 'aborted'
          ? (run.summary?.termination_status === 'aborted_by_circuit_breaker' ? 'TRIPPED' : 'ABORTED')
          : run.status === 'failed'
            ? 'FAILED'
            : 'COMPLETED';
        setStatus(terminalStatus);

        let diag = run.diagnostics || null;
        if (!diag) {
          diag = await runsService.getRunDiagnostics(currentRunId).catch(() => null);
        }
        setDiagnostics(diag);

        if (onRunFinished) {
          onRunFinished(run, diag, terminalStatus);
        }
      } catch (e) {
        console.error('Failed to load completed run:', e);
      }
    };

    const subscription = streamService.subscribeRunStream(
      currentRunId,
      (data: any) => {
        setStreamData(data);
        setChaosActive(!!data.chaos_active);
        setFaultId(data.chaos_fault_id || '');

        let currentStatus = data.status || 'RUNNING';
        if (data.chaos_active) currentStatus = 'INJECTED CHAOS';
        setStatus(currentStatus);

        const upperStatus = currentStatus.toUpperCase();
        if (upperStatus === 'COMPLETED' || upperStatus === 'TRIPPED' || upperStatus === 'ABORTED' || upperStatus === 'FAILED') {
          subscription.close();
          if (timerRef.current) clearInterval(timerRef.current);
          handleRunCompleted();
        }
      },
      () => {
        subscription.close();
        if (timerRef.current) clearInterval(timerRef.current);
        handleRunCompleted();
      }
    );

    return () => {
      subscription.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [currentRunId]);

  return {
    streamData,
    runDetails,
    diagnostics,
    status,
    chaosActive,
    faultId,
    elapsedSec,
  };
}
