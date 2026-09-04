import React, { useState } from 'react';
import { 
  Activity, WarningAltFilled, Close
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { Scenario } from '../types/models';

export interface FlowHeatmapReportProps {
  scenario?: Scenario | null;
  stepMetrics?: Record<string, any>;
  totalRequests?: number;
  onOpenAdvisory?: () => void;
}

export default function FlowHeatmapReport({ scenario, stepMetrics, totalRequests, onOpenAdvisory }: FlowHeatmapReportProps) {
  const { t } = useTranslation();
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null);

  if (!scenario || !scenario.steps) {
    return (
      <div className="p-8 text-center text-[var(--cds-text-helper)] font-mono text-xs">
        {t('heatmap.no_data')}
      </div>
    );
  }

  const steps = Object.values(scenario.steps);
  const metrics = stepMetrics || {};

  // Find the primary bottleneck (step with highest P99 latency or highest errors)
  let primaryBottleneckId: string | null = null;
  let maxP99 = 0;
  let maxErrors = 0;

  steps.forEach((st: any) => {
    const sm = metrics[st.id];
    if (sm) {
      const p99 = sm.latency?.p99 || 0;
      const errs = sm.total_errors || 0;
      if (errs > maxErrors || (errs === maxErrors && p99 > maxP99)) {
        maxErrors = errs;
        maxP99 = p99;
        primaryBottleneckId = st.id;
      }
    }
  });

  const selectedStep: any = steps.find((s: any) => s.id === selectedStepId);
  const selectedStepMetrics = selectedStep ? metrics[selectedStep.id] : null;

  // Compute node heatmap styling
  const getNodeHeatmap = (stepId: string) => {
    const sm = metrics[stepId];
    if (!sm || sm.total_requests === 0) {
      return {
        bg: 'bg-[var(--cds-surface-sunken)] border-[var(--cds-border-subtle)]',
        badge: 'bg-[var(--cds-border-subtle)] text-[var(--cds-text-helper)] border-[var(--cds-layer-03)]',
        status: 'UNTESTED',
        isCritical: false
      };
    }

    const p99Ms = sm.latency.p99 / 1e6;
    const isError = sm.total_errors > 0 || sm.error_rate > 0.05;

    if (isError || p99Ms > 1500) {
      return {
        bg: 'bg-[var(--cds-support-error)]/15 border-[var(--cds-support-error)]',
        badge: 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]',
        status: isError ? 'HIGH ERROR' : 'SLA BREACH',
        isCritical: true
      };
    } else if (p99Ms > 500 || sm.error_rate > 0) {
      return {
        bg: 'bg-[var(--cds-support-warning)]/15 border-[var(--cds-support-warning)]',
        badge: 'bg-[var(--cds-support-warning)]/20 text-[var(--cds-support-warning)] border-[var(--cds-support-warning)]',
        status: 'WARNING',
        isCritical: false
      };
    }

    return {
      bg: 'bg-[var(--cds-support-success)]/15 border-[var(--cds-support-success)]',
      badge: 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]',
      status: 'HEALTHY',
      isCritical: false
    };
  };

  const getMethodBadge = (m?: string) => {
    switch (m) {
      case 'POST': return 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]';
      case 'GET': return 'bg-[var(--cds-interactive)]/20 text-[var(--cds-link)] border-[var(--cds-interactive)]';
      case 'PUT': return 'bg-[var(--cds-support-warning)]/20 text-[var(--cds-support-warning)] border-[var(--cds-support-warning)]';
      case 'DELETE': return 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]';
      default: return 'bg-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)] border-[var(--cds-layer-03)]';
    }
  };

  return (
    <div className="space-y-4 font-mono text-xs">
      
      {/* Legend & Summary Banner */}
      <div className="p-3 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center space-x-4">
          <span className="text-[11px] font-bold text-white uppercase flex items-center gap-1.5">
            <Activity className="w-4 h-4 text-[var(--cds-interactive)]" />
            <span>{t('heatmap.title')}</span>
          </span>
          <div className="flex items-center space-x-3 text-[10px]">
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full bg-[var(--cds-support-success)]"></span> {t('heatmap.legend_optimal')}
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full bg-[var(--cds-support-warning)]"></span> {t('heatmap.legend_near_limit')}
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full bg-[var(--cds-support-error)]"></span> {t('heatmap.legend_bottleneck')}
            </span>
          </div>
        </div>

        {primaryBottleneckId && (
          <div className="flex items-center space-x-2 text-[11px] text-[var(--cds-support-error-text)] bg-[var(--cds-support-error)]/15 px-3 py-1 border border-[var(--cds-support-error)]">
            <WarningAltFilled size={16} className="shrink-0" />
            <span>{t('heatmap.primary_chokepoint', 'Primary Chokepoint: %s', [primaryBottleneckId])}</span>
          </div>
        )}
      </div>

      {/* Heatmap Visual Flow Canvas */}
      <div className="relative w-full h-[580px] bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] overflow-auto flow-grid-bg select-none">
        {/* SVG Flow Arrows with traffic volume stats */}
        <svg className="absolute inset-0 w-[2400px] h-[1600px] pointer-events-none z-0">
          <defs>
            <marker id="arrow-green" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--cds-support-success)" />
            </marker>
            <marker id="arrow-red" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--cds-support-error)" />
            </marker>
            <marker id="arrow-gray" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--cds-border-strong)" />
            </marker>
          </defs>

          {steps.map((sourceStep: any, idx: number) => {
            const sm = metrics[sourceStep.id];
            const sourceX = 80 + (idx % 4) * 440;
            const sourceY = 140 + Math.floor(idx / 4) * 240;

            return (sourceStep.transitions || []).map((trans: any, tIdx: number) => {
              const targetIdx = steps.findIndex((s: any) => s.id === trans.target_step_id);
              if (targetIdx === -1) return null;

              const targetX = 80 + (targetIdx % 4) * 440;
              const targetY = 140 + Math.floor(targetIdx / 4) * 240;

              const startX = sourceX + 320;
              const startY = sourceY + 65;
              const endX = targetX;
              const endY = targetY + 65;

              const dx = Math.max(40, Math.abs(endX - startX) * 0.4);
              const pathD = `M ${startX} ${startY} C ${startX + dx} ${startY}, ${endX - dx} ${endY}, ${endX} ${endY}`;
              const midX = (startX + endX) / 2;
              const midY = (startY + endY) / 2;

              const weightPct = trans.probability ? `${(trans.probability * 100).toFixed(0)}%` : '100%';
              const estHits = sm ? Math.round(sm.total_requests * (trans.probability || 1.0)) : '-';

              return (
                <g key={`${sourceStep.id}-${trans.target_step_id}-${tIdx}`}>
                  <path 
                    d={pathD} 
                    fill="none" 
                    stroke="var(--cds-layer-03)" 
                    strokeWidth="2"
                    markerEnd="url(#arrow-gray)"
                  />
                  {/* Traffic Volume & Percentage badge */}
                  <rect 
                    x={midX - 32} 
                    y={midY - 10} 
                    width="64" 
                    height="20" 
                    rx="2" 
                    fill="var(--cds-layer-01)" 
                    stroke="var(--cds-border-subtle)"
                  />
                  <text 
                    x={midX} 
                    y={midY + 4} 
                    textAnchor="middle" 
                    fill="var(--cds-support-success)" 
                    fontSize="9" 
                    fontFamily="IBM Plex Mono"
                  >
                    {estHits} req ({weightPct})
                  </text>
                </g>
              );
            });
          })}
        </svg>

        {/* Heatmap Step Cards */}
        {steps.map((step: any, idx: number) => {
          const sm = metrics[step.id];
          const heatmap = getNodeHeatmap(step.id);
          const isSelected = selectedStepId === step.id;
          const isBottleneck = primaryBottleneckId === step.id;

          const cardX = 80 + (idx % 4) * 440;
          const cardY = 140 + Math.floor(idx / 4) * 240;

          const p99Ms = sm ? (sm.latency.p99 / 1e6).toFixed(1) : '-';
          const meanMs = sm ? (sm.latency.mean / 1e6).toFixed(1) : '-';
          const reqCount = sm ? sm.total_requests : 0;
          const errCount = sm ? sm.total_errors : 0;
          const errRate = sm ? (sm.error_rate * 100).toFixed(1) : '0.0';

          return (
            <div
              key={step.id}
              onClick={() => setSelectedStepId(step.id)}
              style={{ left: `${cardX}px`, top: `${cardY}px` }}
              className={`absolute w-80 border transition-all cursor-pointer z-10 ${heatmap.bg} ${
                isSelected 
                  ? 'ring-2 ring-[var(--cds-interactive)] shadow-[0_0_20px_rgba(15,98,254,0.5)] scale-[1.02]' 
                  : 'hover:scale-[1.01]'
              } ${isBottleneck ? 'ring-2 ring-[var(--cds-support-error)] shadow-[0_0_20px_rgba(218,30,40,0.6)]' : ''}`}
            >
              {/* Card Header */}
              <div className="p-3 bg-[var(--cds-surface-header)]/80 border-b border-[var(--cds-border-subtle)] flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <span className={`text-[10px] font-bold px-1.5 py-0.5 border ${getMethodBadge(step.request?.method || 'GET')}`}>
                    {step.request?.method || 'GET'}
                  </span>
                  <span className="font-semibold text-xs text-white truncate max-w-[140px]" title={step.name || step.id}>
                    {step.name || step.id}
                  </span>
                </div>
                <div className="flex items-center space-x-1.5">
                  <span className={`text-[9px] px-1.5 py-0.5 border font-bold ${heatmap.badge}`}>
                    {heatmap.status}
                  </span>
                </div>
              </div>

              {/* Card Body with Heatmap Metrics */}
              <div className="p-3.5 space-y-2.5">
                <div className="bg-[var(--cds-background)]/70 p-1.5 text-[11px] text-[var(--cds-support-success)] truncate border border-[var(--cds-border-subtle)]">
                  {step.request?.path || '/'}
                </div>

                {/* Metrics Grid */}
                <div className="grid grid-cols-2 gap-2 pt-1">
                  <div className="bg-[var(--cds-background)]/60 p-2 border border-[var(--cds-border-subtle)]">
                    <div className="text-[9px] text-[var(--cds-text-helper)] uppercase">{t('heatmap.volume', 'VOLUME')}</div>
                    <div className="text-sm font-bold text-white mt-0.5">{reqCount} reqs</div>
                  </div>
                  <div className="bg-[var(--cds-background)]/60 p-2 border border-[var(--cds-border-subtle)]">
                    <div className="text-[9px] text-[var(--cds-text-helper)] uppercase">{t('heatmap.p99_latency', 'P99 LATENCY')}</div>
                    <div className={`text-sm font-bold mt-0.5 ${sm && (sm.latency.p99 / 1e6) > 1000 ? 'text-[var(--cds-support-error-text)]' : 'text-white'}`}>
                      {p99Ms} ms
                    </div>
                  </div>
                </div>

                {/* Error & Mean Latency Row */}
                <div className="flex items-center justify-between text-[10px] pt-1 border-t border-[var(--cds-border-subtle)]/70 text-[var(--cds-text-helper)]">
                  <span>{t('heatmap.mean_label', 'Mean:')} <strong className="text-white">{meanMs}ms</strong></span>
                  <span>{t('heatmap.errors_label', 'Errors:')} <strong className={errCount > 0 ? 'text-[var(--cds-support-error-text)]' : 'text-[var(--cds-support-success-text)]'}>{errCount} ({errRate}%)</strong></span>
                </div>

                {/* Bottleneck Callout */}
                {isBottleneck && (
                  <div className="p-1.5 bg-[var(--cds-support-error)]/25 border border-[var(--cds-support-error)] text-[10px] text-[var(--cds-support-error-text)] flex items-center justify-center gap-1 font-bold">
                    <WarningAltFilled size={14} />
                    <span>{t('heatmap.bottleneck_badge', 'PRIMARY BOTTLENECK')}</span>
                  </div>
                )}
              </div>
            </div>
          );
        })}

        {/* Terminal END Node */}
        <div 
          style={{ left: `${80 + (steps.length % 4) * 440}px`, top: `${140 + Math.floor(steps.length / 4) * 240}px` }}
          className="absolute w-28 p-3 bg-[var(--cds-background)] border-2 border-dashed border-[var(--cds-layer-03)] text-center font-mono text-xs text-[var(--cds-text-helper)]"
        >
          <div className="text-[10px] text-[var(--cds-text-primary)] font-bold">{t('heatmap.terminal_label')}</div>
          <div className="text-sm font-bold text-[var(--cds-support-success)] mt-0.5">{t('heatmap.terminal_end')}</div>
        </div>
      </div>

      {/* Step Metric Drilldown Drawer (Opens when card clicked) */}
      {selectedStep && (
        <div className="p-4 bg-[var(--cds-layer-01)] border border-[var(--cds-interactive)] space-y-3">
          <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
            <div className="flex items-center space-x-2">
              <span className={`text-[10px] px-1.5 py-0.5 border ${getMethodBadge(selectedStep.request?.method || 'GET')}`}>
                {selectedStep.request?.method || 'GET'}
              </span>
              <span className="font-bold text-sm text-white">{t('heatmap.step_drilldown', 'Step Drilldown: %s', [selectedStep.name || selectedStep.id])}</span>
              <span className="text-xs text-[var(--cds-text-helper)]">({selectedStep.request?.path || '/'})</span>
            </div>
            <button 
              onClick={() => setSelectedStepId(null)}
              className="text-[var(--cds-text-helper)] hover:text-white p-1"
            >
              <Close size={16} />
            </button>
          </div>

          {selectedStepMetrics ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Latency Quantile Breakdown */}
              <div className="space-y-2">
                <span className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold">{t('heatmap.quantile_breakdown')}</span>
                <div className="overflow-x-auto border border-[var(--cds-border-subtle)]">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-[var(--cds-surface-sunken)] text-[var(--cds-text-helper)] border-b border-[var(--cds-border-subtle)]">
                      <tr>
                        <th className="p-1.5">{t('heatmap.col_min', 'MIN')}</th>
                        <th className="p-1.5">{t('heatmap.col_mean', 'MEAN')}</th>
                        <th className="p-1.5">P50</th>
                        <th className="p-1.5">P90</th>
                        <th className="p-1.5 text-[var(--cds-support-error)]">P99</th>
                        <th className="p-1.5">{t('heatmap.col_max', 'MAX')}</th>
                      </tr>
                    </thead>
                    <tbody className="bg-[var(--cds-background)] text-[var(--cds-text-secondary)]">
                      <tr>
                        <td className="p-1.5">{(selectedStepMetrics.latency.min / 1e6).toFixed(1)}ms</td>
                        <td className="p-1.5">{(selectedStepMetrics.latency.mean / 1e6).toFixed(1)}ms</td>
                        <td className="p-1.5">{(selectedStepMetrics.latency.p50 / 1e6).toFixed(1)}ms</td>
                        <td className="p-1.5">{(selectedStepMetrics.latency.p90 / 1e6).toFixed(1)}ms</td>
                        <td className="p-1.5 text-[var(--cds-support-error)] font-bold">{(selectedStepMetrics.latency.p99 / 1e6).toFixed(1)}ms</td>
                        <td className="p-1.5">{(selectedStepMetrics.latency.max / 1e6).toFixed(1)}ms</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Status Codes Distribution */}
              <div className="space-y-2">
                <span className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold">{t('heatmap.status_codes')}</span>
                <div className="flex flex-wrap gap-2 p-2.5 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  {selectedStepMetrics.status_codes && Object.keys(selectedStepMetrics.status_codes).length > 0 ? (
                    Object.entries(selectedStepMetrics.status_codes).map(([code, count]: [string, any]) => (
                      <span key={code} className="px-2 py-0.5 border text-xs bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]">
                        HTTP {code}: {count}
                      </span>
                    ))
                  ) : (
                    <span className="text-[var(--cds-text-helper)]">{t('heatmap.no_status_records')}</span>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="p-4 text-center text-[var(--cds-text-helper)]">{t('heatmap.no_metrics_step')}</div>
          )}
        </div>
      )}

    </div>
  );
}
