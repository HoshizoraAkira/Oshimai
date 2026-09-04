import React, { useEffect, useRef, useState } from 'react';
import Chart from 'chart.js/auto';
import { 
  Activity, StopFilled, CheckmarkFilled, 
  Time, Certificate, Flash, Document, ArrowUpRight
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { streamService } from '../services/streamService';
import { CARBON_CHART_OPTIONS, createDualAxisOptions } from '../constants/chartConfig';
import { nsToMs } from '../utils/units';
import { ToastType, Run, RunDiagnostics } from '../types/models';
import FlowHeatmapReport from './FlowHeatmapReport';

export interface LiveCockpitProps {
  currentRunId: string | null;
  onAbortRun: (runId: string) => void;
  onOpenSummaryDrawer: (runId: string) => void;
  onOpenDoctorModal: (runId: string) => void;
  onOpenComplianceCertificate?: (run: any) => void;
  onRunActiveChange?: (isActive: boolean) => void;
  showToast: (msg: string, type?: ToastType) => void;
  activeSubTab?: string;
}

export default function LiveCockpit({
  currentRunId,
  onAbortRun,
  onOpenSummaryDrawer,
  onOpenDoctorModal,
  onOpenComplianceCertificate,
  onRunActiveChange,
  showToast,
  activeSubTab,
}: LiveCockpitProps) {
  const { t } = useTranslation();
  const incidentBarRef = useRef<HTMLDivElement>(null);
  const [incidentFlash, setIncidentFlash] = useState(false);

  useEffect(() => {
    if (activeSubTab !== 'incidents') return;
    incidentBarRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setIncidentFlash(true);
    const timer = setTimeout(() => setIncidentFlash(false), 1500);
    return () => clearTimeout(timer);
  }, [activeSubTab]);

  const [streamData, setStreamData] = useState<any>(null);
  const [runDetails, setRunDetails] = useState<Run | null>(null);
  const [diagnostics, setDiagnostics] = useState<RunDiagnostics | null>(null);
  const [reportView, setReportView] = useState<'flow' | 'table'>('flow');
  const [status, setStatus] = useState('IDLE');
  const [chaosActive, setChaosActive] = useState(false);
  const [faultId, setFaultId] = useState('');
  const [elapsedSec, setElapsedSec] = useState(0);

  const chartThroughputRef = useRef<HTMLCanvasElement>(null);
  const chartLatencyRef = useRef<HTMLCanvasElement>(null);
  const chartErrorRef = useRef<HTMLCanvasElement>(null);

  const throughputChartInst = useRef<any>(null);
  const latencyChartInst = useRef<any>(null);
  const errorChartInst = useRef<any>(null);
  const timerRef = useRef<any>(null);

  // Initialize Charts using shared Carbon design configuration
  useEffect(() => {
    if (chartThroughputRef.current) {
      throughputChartInst.current = new Chart(chartThroughputRef.current, {
        type: 'line',
        data: {
          labels: [],
          datasets: [
            { label: t('cockpit.throughput_rps', 'Throughput (RPS)'), borderColor: 'var(--cds-interactive)', backgroundColor: 'rgba(15,98,254,0.1)', data: [], tension: 0.1, fill: true },
            { label: t('cockpit.active_vus', 'Active Virtual Users'), borderColor: 'var(--cds-support-novel-strong)', backgroundColor: 'transparent', data: [], borderDash: [4,4], tension: 0.1 }
          ]
        },
        options: CARBON_CHART_OPTIONS as any
      });
    }

    if (chartLatencyRef.current) {
      latencyChartInst.current = new Chart(chartLatencyRef.current, {
        type: 'line',
        data: {
          labels: [],
          datasets: [
            { label: 'P50 (ms)', borderColor: 'var(--cds-support-success)', data: [], tension: 0.1, borderWidth: 1.5 },
            { label: 'P90 (ms)', borderColor: 'var(--cds-support-warning)', data: [], tension: 0.1, borderWidth: 1.5 },
            { label: 'P99 SLA (ms)', borderColor: 'var(--cds-support-error)', backgroundColor: 'rgba(218,30,40,0.08)', data: [], tension: 0.1, fill: true, borderWidth: 2 }
          ]
        },
        options: CARBON_CHART_OPTIONS as any
      });
    }

    if (chartErrorRef.current) {
      errorChartInst.current = new Chart(chartErrorRef.current, {
        type: 'line',
        data: {
          labels: [],
          datasets: [
            { label: t('cockpit.error_rate_count', 'Error Rate (%)'), borderColor: 'var(--cds-support-warning)', backgroundColor: 'rgba(241,194,27,0.15)', data: [], tension: 0.1, fill: true, yAxisID: 'y' },
            { label: 'Error Count', borderColor: 'var(--cds-support-info)', data: [], borderDash: [3,3], tension: 0.1, yAxisID: 'y1' }
          ]
        },
        options: createDualAxisOptions('var(--cds-support-info)') as any
      });
    }

    return () => {
      throughputChartInst.current?.destroy();
      latencyChartInst.current?.destroy();
      errorChartInst.current?.destroy();
    };
  }, []);

  // Fetch final run summary & diagnostics on completion
  const handleRunCompleted = async (runId: string) => {
    try {
      const run = await runsService.getRun(runId);
      setRunDetails(run);

      const terminalStatus = run.status === 'aborted'
        ? (run.summary?.termination_status === 'aborted_by_circuit_breaker' ? 'TRIPPED' : 'ABORTED')
        : run.status === 'failed'
          ? 'FAILED'
          : 'COMPLETED';
      setStatus(terminalStatus);

      // Fetch Diagnostics
      let diag = (run as any).diagnostics;
      if (!diag) {
        diag = await runsService.getRunDiagnostics(runId).catch(() => null);
      }
      setDiagnostics(diag);
      const toastMsg = terminalStatus === 'FAILED' && run.error
        ? `Test Run ${runId} failed: ${run.error}`
        : `Test Run ${runId} finalized with status ${terminalStatus}!`;
      showToast(toastMsg, terminalStatus === 'COMPLETED' ? 'success' : (terminalStatus === 'FAILED' ? 'error' : 'warning'));
    } catch (e) {
      console.error('Error loading final run report:', e);
    }
  };

  // Connect SSE Telemetry via streamService
  useEffect(() => {
    if (!currentRunId) {
      setStatus('IDLE');
      return;
    }

    setStatus('RUNNING');
    setElapsedSec(0);
    setRunDetails(null);
    setDiagnostics(null);

    // Reset charts
    [throughputChartInst.current, latencyChartInst.current, errorChartInst.current].forEach(chart => {
      if (chart) {
        chart.data.labels = [];
        chart.data.datasets.forEach((ds: any) => ds.data = []);
        chart.update();
      }
    });

    // Start timer
    if (timerRef.current) clearInterval(timerRef.current);
    const startMs = Date.now();
    timerRef.current = setInterval(() => {
      setElapsedSec(Math.floor((Date.now() - startMs) / 1000));
    }, 1000);

    let cancelled = false;
    let subscription: { close: () => void } | null = null;

    const TERMINAL_STATUSES = ['completed', 'aborted', 'failed'];

    const start = async () => {
      // The event bus does not replay past events, so subscribing to the stream of a run that
      // already finished before this mount (e.g. a page reload, or reopening a past run from
      // History) would just hang forever waiting for events that will never arrive. Check the
      // run's persisted status first and skip straight to the final report in that case.
      try {
        const existing = await runsService.getRun(currentRunId);
        if (cancelled) return;
        if (TERMINAL_STATUSES.includes(existing.status)) {
          if (timerRef.current) clearInterval(timerRef.current);
          handleRunCompleted(currentRunId);
          return;
        }
      } catch {
        // Transient lookup failure — fall through and try the live stream anyway.
      }
      if (cancelled) return;

      subscription = streamService.subscribeRunStream(
      currentRunId,
      (data) => {
        setStreamData(data);
        setChaosActive(data.chaos_active);
        setFaultId(data.chaos_fault_id || '');

        let currentStatus = data.status || 'RUNNING';
        if (data.chaos_active) currentStatus = 'INJECTED CHAOS';
        setStatus(currentStatus);

        const timeLabel = new Date().toTimeString().split(' ')[0].slice(3, 8);
        const maxDataPoints = 30;

        const appendPoint = (chart: any, datasetIdx: number, val: number) => {
          if (!chart) return;
          if (datasetIdx === 0) {
            chart.data.labels.push(timeLabel);
            if (chart.data.labels.length > maxDataPoints) chart.data.labels.shift();
          }
          if (chart.data.datasets[datasetIdx]) {
            chart.data.datasets[datasetIdx].data.push(val);
            if (chart.data.datasets[datasetIdx].data.length > maxDataPoints) {
              chart.data.datasets[datasetIdx].data.shift();
            }
          }
        };

        const rps = data.current_rps || 0;
        const activeVus = data.active_vus || 0;
        const p50 = nsToMs(data.latency_p50);
        const p90 = nsToMs(data.latency_p90);
        const p99 = nsToMs(data.latency_p99);
        const errRate = (data.error_rate || 0) * 100;
        const errCount = data.error_count || 0;

        appendPoint(throughputChartInst.current, 0, rps);
        appendPoint(throughputChartInst.current, 1, activeVus);

        appendPoint(latencyChartInst.current, 0, p50);
        appendPoint(latencyChartInst.current, 1, p90);
        appendPoint(latencyChartInst.current, 2, p99);

        appendPoint(errorChartInst.current, 0, errRate);
        appendPoint(errorChartInst.current, 1, errCount);

        throughputChartInst.current?.update();
        latencyChartInst.current?.update();
        errorChartInst.current?.update();

        // Terminal status received
        const upperStatus = currentStatus.toUpperCase();
        if (upperStatus === 'COMPLETED' || upperStatus === 'TRIPPED' || upperStatus === 'ABORTED' || upperStatus === 'FAILED') {
          subscription?.close();
          handleRunCompleted(currentRunId);
        }
      },
      () => {
        subscription?.close();
        if (timerRef.current) clearInterval(timerRef.current);
        handleRunCompleted(currentRunId);
      }
      );
    };

    start();

    return () => {
      cancelled = true;
      subscription?.close();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [currentRunId]);

  const getStatusBadgeClass = () => {
    switch (status) {
      case 'RUNNING': return 'cds-badge-info';
      case 'INJECTED CHAOS': return 'cds-badge-novel';
      case 'DRAINING': return 'cds-badge-warning';
      case 'COMPLETED': return 'cds-badge-success';
      case 'TRIPPED':
      case 'ABORTED':
      case 'FAILED': return 'cds-badge-error';
      default: return 'cds-badge-neutral';
    }
  };

  const formatSec = (s: number) => {
    const m = String(Math.floor(s / 60)).padStart(2, '0');
    const sec = String(s % 60).padStart(2, '0');
    return `${m}:${sec}`;
  };

  const isCompletedOrAborted = status === 'COMPLETED' || status === 'TRIPPED' || status === 'ABORTED' || status === 'FAILED';

  // Report whether the current run is genuinely still active so ancestors (e.g. the
  // Admin Dock's kill-switch) don't keep treating a finished run as live just because
  // a runId is still referenced somewhere.
  useEffect(() => {
    onRunActiveChange?.(!!currentRunId && !isCompletedOrAborted && status !== 'IDLE');
  }, [currentRunId, status, isCompletedOrAborted, onRunActiveChange]);

  return (
    <div className="space-y-6">

      {/* Live Session Bar */}
      <div
        ref={incidentBarRef}
        className={`cds-tile p-4 flex flex-wrap items-center justify-between gap-4 border-l-4 border-l-[var(--cds-interactive)] transition-shadow ${incidentFlash ? 'ring-2 ring-[var(--cds-support-warning)]' : ''}`}
      >
        <div className="flex items-center gap-4">
          <div>
            <div className="cds-tile-eyebrow">{t('cockpit.telemetry_session')}</div>
            <div className="text-sm font-mono font-semibold text-[var(--cds-text-primary)] mt-0.5">
              {currentRunId || t('cockpit.no_active_run')}
            </div>
          </div>
          <div className="text-xs font-mono text-[var(--cds-text-helper)] flex items-center gap-1.5">
            <Time size={14} />
            <span>{formatSec(elapsedSec)}</span>
          </div>
        </div>

        <div className="flex items-center gap-2 font-mono">
          <span className={getStatusBadgeClass()}>{status}</span>

          <span className={chaosActive ? 'cds-badge-error' : 'cds-badge-neutral'}>
            {chaosActive ? t('cockpit.fault_active', 'FAULT: %s', [faultId || 'NETEM']) : t('cockpit.no_fault')}
          </span>

          <button
            disabled={!currentRunId || isCompletedOrAborted}
            onClick={() => currentRunId && onAbortRun(currentRunId)}
            className="cds-btn-danger cds-btn-sm"
          >
            <StopFilled size={14} />
            <span>{t('cockpit.abort_run')}</span>
          </button>
        </div>
      </div>

      {/* 4 Real-Time KPI Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 font-mono">
        <div className="cds-tile p-3.5">
          <div className="cds-tile-eyebrow">{t('cockpit.throughput_rps')}</div>
          <div className="text-2xl font-semibold text-[var(--cds-interactive)] mt-1">
            {isCompletedOrAborted && runDetails?.summary
              ? runDetails.summary.actual_rps.toFixed(1)
              : (streamData?.current_rps?.toFixed(1) || '0.0')}
          </div>
        </div>
        <div className="cds-tile p-3.5">
          <div className="cds-tile-eyebrow">{t('cockpit.active_vus')}</div>
          <div className="text-2xl font-semibold text-[var(--cds-support-novel)] mt-1">
            {isCompletedOrAborted
              ? (runDetails?.config?.load_config?.vus || 0)
              : (streamData?.active_vus || 0)}
          </div>
        </div>
        <div className="cds-tile p-3.5">
          <div className="cds-tile-eyebrow">{t('cockpit.p99_latency_sla')}</div>
          <div className="text-2xl font-semibold text-[var(--cds-support-error)] mt-1">
            {isCompletedOrAborted && runDetails?.summary
              ? `${(runDetails.summary.latency.p99 / 1e6).toFixed(1)} ms`
              : (streamData?.latency_p99 ? `${(streamData.latency_p99 / 1e6).toFixed(1)} ms` : '0.0 ms')}
          </div>
        </div>
        <div className="cds-tile p-3.5">
          <div className="cds-tile-eyebrow">{t('cockpit.error_rate_count')}</div>
          <div className="text-2xl font-semibold text-[var(--cds-support-warning)] mt-1">
            {isCompletedOrAborted && runDetails?.summary
              ? `${((runDetails.summary.total_errors / Math.max(1, runDetails.summary.total_requests)) * 100).toFixed(1)}%`
              : (streamData?.error_rate ? `${(streamData.error_rate * 100).toFixed(1)}%` : '0.0%')}
            <span className="text-xs text-[var(--cds-text-helper)] ml-1">
              ({isCompletedOrAborted && runDetails?.summary ? runDetails.summary.total_errors : (streamData?.error_count || 0)} err)
            </span>
          </div>
        </div>
      </div>

      {/* AUTOMATED POST-RUN REPORT SECTION */}
      {isCompletedOrAborted && (
        <div className="bg-[var(--cds-background)] border-2 border-[var(--cds-support-success)] p-5 space-y-4 font-mono shadow-[0_4px_24px_rgba(36,161,72,0.15)]">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--cds-border-subtle)] pb-3">
            <div className="flex items-center gap-2.5">
              <CheckmarkFilled size={18} className="text-[var(--cds-support-success)]" />
              <div>
                <div className="text-xs text-[var(--cds-support-success)] font-bold uppercase tracking-wider">{t('cockpit.completed_report_title')}</div>
                <div className="text-sm font-semibold text-[var(--cds-text-primary)]">
                  {diagnostics?.status_label || (status === 'COMPLETED' ? t('cockpit.app_resilient') : t('cockpit.test_finished'))}
                </div>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <div className="inline-flex border border-[var(--cds-border-subtle)] bg-[var(--cds-background)]">
                <button
                  onClick={() => setReportView('flow')}
                  className={`px-3 py-1.5 text-xs flex items-center gap-1.5 ${reportView === 'flow' ? 'bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] font-bold' : 'text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'}`}
                >
                  <Activity size={14} className="text-[var(--cds-support-success)]" />
                  <span>{t('cockpit.tab_flow')}</span>
                </button>
                <button
                  onClick={() => setReportView('table')}
                  className={`px-3 py-1.5 text-xs flex items-center gap-1.5 border-l border-[var(--cds-border-subtle)] ${reportView === 'table' ? 'bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] font-bold' : 'text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'}`}
                >
                  <Document size={14} className="text-[var(--cds-interactive)]" />
                  <span>{t('cockpit.tab_cards')}</span>
                </button>
              </div>

              <button
                onClick={() => onOpenComplianceCertificate && onOpenComplianceCertificate(runDetails || { id: currentRunId, summary: runDetails?.summary, diagnostics })}
                className="cds-btn-success cds-btn-sm normal-case"
              >
                <Certificate size={14} />
                <span>{t('cockpit.compliance_cert_btn')}</span>
              </button>

              <button
                onClick={() => currentRunId && onOpenSummaryDrawer(currentRunId)}
                className="cds-btn-primary cds-btn-sm normal-case"
              >
                <Document size={14} />
                <span>{t('cockpit.full_telemetry_btn')}</span>
              </button>
              <button
                onClick={() => currentRunId && onOpenDoctorModal(currentRunId)}
                className="cds-btn-secondary cds-btn-sm normal-case"
              >
                <Flash size={14} className="text-[var(--cds-support-warning)]" />
                <span>{t('cockpit.remediation_btn')}</span>
              </button>
            </div>
          </div>

          {reportView === 'flow' && (
            <FlowHeatmapReport
              scenario={runDetails?.scenario}
              stepMetrics={runDetails?.summary?.step_metrics}
              totalRequests={runDetails?.summary?.total_requests}
              onOpenAdvisory={() => currentRunId && onOpenDoctorModal(currentRunId)}
            />
          )}

          {reportView === 'table' && (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="cds-tile p-3.5 space-y-1.5">
                <div className="cds-tile-eyebrow">{t('cockpit.resilience_assessment')}</div>
                <div className="text-sm font-semibold text-[var(--cds-text-primary)]">
                  {diagnostics?.status_label || t('cockpit.optimal_health')}
                </div>
                <p className="text-xs text-[var(--cds-text-secondary)] leading-relaxed font-sans">
                  {diagnostics?.summary || t('cockpit.processed_summary', 'Total %s requests processed with actual rate of %s RPS.', [runDetails?.summary?.total_requests || 0, (runDetails?.summary?.actual_rps || 0).toFixed(1)])}
                </p>
              </div>

              <div className="cds-tile p-3.5 space-y-1.5">
                <div className="flex items-center justify-between">
                  <span className="cds-tile-eyebrow">{t('cockpit.health_score')}</span>
                  <span className={(diagnostics?.health_score ?? 100) >= 85 ? 'cds-badge-success' : 'cds-badge-warning'}>
                    {(diagnostics?.health_score ?? 100) >= 85 ? t('cockpit.score_optimal') : t('cockpit.score_degraded')}
                  </span>
                </div>
                <div className="flex items-baseline gap-1.5">
                  <span className="text-3xl font-bold text-[var(--cds-support-success)]">{diagnostics?.health_score || 100}</span>
                  <span className="text-xs text-[var(--cds-text-helper)]">/ 100</span>
                </div>
                <p className="text-[11px] text-[var(--cds-text-helper)] font-sans">
                  {diagnostics?.summary_short || t('cockpit.score_eval')}
                </p>
              </div>

              <div className="cds-tile p-3.5 space-y-2">
                <div className="cds-tile-eyebrow">{t('cockpit.doctor_advisory')}</div>
                <div className="text-xs font-semibold text-[var(--cds-support-warning)] flex items-center gap-1">
                  <Flash size={14} />
                  <span>{t('cockpit.root_cause_found', '%s Root-Cause Defect(s) Found', [(diagnostics as any)?.remediations?.length || 0])}</span>
                </div>
                <p className="text-[11px] text-[var(--cds-text-secondary)] line-clamp-2 font-sans">
                  {(diagnostics as any)?.remediations?.[0]?.description || t('cockpit.no_advisory')}
                </p>
                <button
                  onClick={() => currentRunId && onOpenDoctorModal(currentRunId)}
                  className="cds-btn-ghost cds-btn-sm p-0 text-[var(--cds-interactive)] normal-case flex items-center gap-1"
                >
                  <span>{t('cockpit.view_remediation')}</span>
                  <ArrowUpRight size={14} />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Real-time Telemetry Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="cds-tile p-3.5 space-y-2 flex flex-col">
          <div className="flex items-center justify-between">
            <span className="cds-tile-eyebrow">{t('cockpit.chart_throughput')}</span>
            <span className="text-[11px] font-mono text-[var(--cds-interactive)]">{t('cockpit.chart_live_500ms')}</span>
          </div>
          <div className="h-56 w-full relative">
            <canvas ref={chartThroughputRef} />
          </div>
        </div>

        <div className="cds-tile p-3.5 space-y-2 flex flex-col">
          <div className="flex items-center justify-between">
            <span className="cds-tile-eyebrow">{t('cockpit.chart_latency')}</span>
            <span className="text-[11px] font-mono text-[var(--cds-support-error)]">{t('cockpit.chart_sla_p99')}</span>
          </div>
          <div className="h-56 w-full relative">
            <canvas ref={chartLatencyRef} />
          </div>
        </div>

        <div className="cds-tile p-3.5 space-y-2 flex flex-col">
          <div className="flex items-center justify-between">
            <span className="cds-tile-eyebrow">{t('cockpit.chart_errors')}</span>
            <span className="text-[11px] font-mono text-[var(--cds-support-warning)]">{t('cockpit.chart_percent_count')}</span>
          </div>
          <div className="h-56 w-full relative">
            <canvas ref={chartErrorRef} />
          </div>
        </div>
      </div>

    </div>
  );
}
