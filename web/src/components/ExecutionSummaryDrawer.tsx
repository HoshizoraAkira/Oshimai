import React, { useEffect, useState } from 'react';
import { Close, WarningAltFilled, Flash } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { Run } from '../types/models';

export interface ExecutionSummaryDrawerProps {
  runId: string | null;
  isOpen: boolean;
  onClose: () => void;
  onOpenDoctorModal: (runId: string) => void;
}

export default function ExecutionSummaryDrawer({ runId, isOpen, onClose, onOpenDoctorModal }: ExecutionSummaryDrawerProps) {
  const { t } = useTranslation();
  const [run, setRun] = useState<Run | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!runId || !isOpen) return;
    setLoading(true);

    runsService.getRun(runId)
      .then(data => {
        setRun(data);
        setLoading(false);
      })
      .catch(err => {
        console.error('Drawer fetch error:', err);
        setLoading(false);
      });
  }, [runId, isOpen]);

  if (!isOpen) return null;

  const summary = run?.summary;
  const lat = summary?.latency;
  const isAborted = run?.status === 'aborted';

  return (
    <div className="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex justify-end transition-opacity">
      <div className="bg-[var(--cds-layer-01)] border-l border-[var(--cds-border-subtle)] w-full max-w-xl h-full flex flex-col shadow-2xl font-mono">
        
        {/* Header */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between">
          <div>
            <div className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold tracking-wider">
              {t('drawer.title')}
            </div>
            <h2 className="text-sm font-semibold text-white mt-0.5">
              {t('drawer.run_label', 'Run: %s', [runId])}
            </h2>
          </div>
          <button 
            onClick={onClose}
            className="p-1.5 text-[var(--cds-text-helper)] hover:text-white hover:bg-[var(--cds-border-subtle)] transition-colors"
          >
            <Close size={16} />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-5 space-y-5 text-xs">
          {loading ? (
            <div className="p-8 text-center text-[var(--cds-text-helper)]">{t('drawer.loading')}</div>
          ) : run ? (
            <>
              {/* Termination Status & Abort Alert */}
              <div className="p-3.5 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--cds-text-helper)] uppercase">{t('drawer.termination_outcome')}</span>
                  <span className={`px-2 py-0.5 border text-[11px] font-bold ${
                    isAborted ? 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]' : 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]'
                  }`}>
                    {(summary?.termination_status || run.status).toUpperCase()}
                  </span>
                </div>
                {(summary?.abort_reason || (run as any).error) && (
                  <div className="pt-2 border-t border-[var(--cds-border-subtle)] text-[var(--cds-support-error-text)] flex items-start gap-1.5">
                    <WarningAltFilled size={16} className="shrink-0 mt-0.5" />
                    <span>{summary?.abort_reason || (run as any).error}</span>
                  </div>
                )}
              </div>

              {/* 4 Core Metrics */}
              <div className="grid grid-cols-2 gap-3">
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  <div className="text-[var(--cds-text-helper)] text-[10px] uppercase">{t('drawer.total_requests')}</div>
                  <div className="text-xl font-bold text-white mt-1">
                    {summary?.total_requests || 0}
                  </div>
                </div>
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  <div className="text-[var(--cds-text-helper)] text-[10px] uppercase">{t('drawer.total_errors')}</div>
                  <div className="text-xl font-bold text-[var(--cds-support-error)] mt-1">
                    {summary?.total_errors || 0}
                  </div>
                </div>
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  <div className="text-[var(--cds-text-helper)] text-[10px] uppercase">{t('drawer.actual_rps')}</div>
                  <div className="text-xl font-bold text-[var(--cds-interactive)] mt-1">
                    {summary?.actual_rps ? summary.actual_rps.toFixed(1) : '0.0'}
                  </div>
                </div>
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  <div className="text-[var(--cds-text-helper)] text-[10px] uppercase">{t('drawer.total_duration')}</div>
                  <div className="text-xl font-bold text-white mt-1">
                    {summary?.total_duration ? `${(summary.total_duration / 1e9).toFixed(2)}s` : '0s'}
                  </div>
                </div>
              </div>

              {/* Latency Quantile Distribution Table */}
              <div className="space-y-2">
                <span className="text-[11px] text-[var(--cds-interactive)] font-bold uppercase">{t('drawer.hdr_latency')}</span>
                <div className="overflow-x-auto border border-[var(--cds-border-subtle)]">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-[var(--cds-surface-sunken)] text-[var(--cds-text-helper)] border-b border-[var(--cds-border-subtle)]">
                      <tr>
                        <th className="p-2">{t('heatmap.col_min', 'MIN')}</th>
                        <th className="p-2">{t('heatmap.col_mean', 'MEAN')}</th>
                        <th className="p-2">P50</th>
                        <th className="p-2">P90</th>
                        <th className="p-2">P95</th>
                        <th className="p-2 text-[var(--cds-support-error)]">P99 SLA</th>
                        <th className="p-2">{t('heatmap.col_max', 'MAX')}</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--cds-border-subtle)] bg-[var(--cds-background)] text-[var(--cds-text-secondary)]">
                      <tr>
                        <td className="p-2">{lat ? `${(lat.min / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2">{lat ? `${(lat.mean / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2">{lat ? `${(lat.p50 / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2">{lat ? `${(lat.p90 / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2">{lat ? `${(lat.p95 / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2 text-[var(--cds-support-error)] font-bold">{lat ? `${(lat.p99 / 1e6).toFixed(1)}ms` : '-'}</td>
                        <td className="p-2">{lat ? `${(lat.max / 1e6).toFixed(1)}ms` : '-'}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              {/* HTTP Status Codes Breakdown */}
              <div className="space-y-2">
                <span className="text-[11px] text-[var(--cds-interactive)] font-bold uppercase">{t('heatmap.status_codes')}</span>
                <div className="flex flex-wrap gap-2 p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
                  {summary?.status_codes && Object.keys(summary.status_codes).length > 0 ? (
                    Object.entries(summary.status_codes).map(([code, count]) => {
                      const num = parseInt(code);
                      const isErr = num >= 500 || num === 0;
                      const isWarn = num >= 400 && num < 500;
                      const bg = isErr ? 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]' 
                               : isWarn ? 'bg-[var(--cds-support-warning)]/20 text-[var(--cds-support-warning)] border-[var(--cds-support-warning)]' 
                               : 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]';
                      return (
                        <span key={code} className={`px-2 py-1 border text-xs font-semibold ${bg}`}>
                          HTTP {code}: {String(count)}
                        </span>
                      );
                    })
                  ) : (
                    <span className="text-[var(--cds-text-helper)]">{t('heatmap.no_status_records')}</span>
                  )}
                </div>
              </div>
            </>
          ) : (
            <div className="p-8 text-center text-[var(--cds-support-error)]">{t('drawer.run_not_found', 'Run not found')}</div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-t border-[var(--cds-border-subtle)] flex items-center justify-between">
          <button 
            onClick={() => {
              onClose();
              if (runId) onOpenDoctorModal(runId);
            }}
            className="px-3 py-1.5 bg-[var(--cds-border-subtle)] hover:bg-[var(--cds-layer-03)] text-white text-xs flex items-center gap-1.5 transition-colors font-semibold"
          >
            <Flash size={16} className="text-[var(--cds-support-warning)]" />
            <span>{t('drawer.open_remediation')}</span>
          </button>
          <button 
            onClick={onClose}
            className="px-3 py-1.5 bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] text-white text-xs font-semibold"
          >
            {t('common.close')}
          </button>
        </div>

      </div>
    </div>
  );
}
