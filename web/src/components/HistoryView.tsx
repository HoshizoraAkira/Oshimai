import React, { useEffect, useState } from 'react';
import { Reset, Document, Flash, Share, Badge as BadgeIcon, Analytics, Activity } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { Run, ToastType } from '../types/models';

export interface HistoryViewProps {
  onInspectRun: (runId: string) => void;
  onOpenAdvisory: (runId: string) => void;
  onViewCockpit: (runId: string) => void;
  showToast: (msg: string, type?: ToastType) => void;
  activeSubTab?: string;
}

export default function HistoryView({ onInspectRun, onOpenAdvisory, onViewCockpit, showToast, activeSubTab }: HistoryViewProps) {
  const { t, formatTime } = useTranslation();
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(false);
  const [trends, setTrends] = useState<Record<string, any>>({}); // runId -> TrendReport

  const handleShare = async (runId: string) => {
    try {
      const data = await runsService.createShareLink(runId);
      const publicUrl = `${window.location.origin}/?public=${data.share_token}`;
      await navigator.clipboard.writeText(publicUrl).catch(() => {});
      showToast(t('toasts.link_copied', 'Public status link copied: %s', [publicUrl]), 'success');
    } catch (err: any) {
      showToast(t('toasts.share_failed', 'Failed to share: %s', [err.message]), 'error');
    }
  };

  const handleCopyBadge = async (runId: string) => {
    const badgeUrl = `${window.location.origin}/api/v1/runs/${runId}/badge.svg`;
    const markdown = `[![Battle-Tested by Oshimai](${badgeUrl})](${window.location.origin})`;
    await navigator.clipboard.writeText(markdown).catch(() => {});
    showToast(t('toasts.badge_copied', 'Badge markdown copied — paste it in your README.'), 'success');
  };

  const handleCheckTrend = async (runId: string) => {
    try {
      const data = await runsService.getRunTrend(runId);
      setTrends(prev => ({ ...prev, [runId]: data }));
    } catch (err: any) {
      showToast(err.message || t('toasts.trend_failed', 'Failed to check trend: %s', [err.message]), 'warning');
    }
  };

  const fetchRuns = async () => {
    setLoading(true);
    try {
      const data = await runsService.listRuns({ limit: 30 });
      setRuns(data || []);
    } catch (err: any) {
      showToast(t('toasts.fetch_history_failed', 'Failed to fetch history: %s', [err.message]), 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRuns();
  }, []);

  // "Baseline & Trends" submenu has no separate page — trigger the per-row trend
  // check for every listed run instead of leaving the click with no visible effect.
  useEffect(() => {
    if (activeSubTab !== 'trends' || runs.length === 0) return;
    const pending = runs.filter(run => !trends[run.id]);
    if (pending.length === 0) return;
    showToast(t('toasts.checking_trends', 'Checking baseline trend for %s run(s)...', [pending.length]), 'info');
    pending.forEach(run => handleCheckTrend(run.id));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeSubTab, runs]);

  const getStatusBadge = (status?: string) => {
    switch (status) {
      case 'completed': return 'cds-badge-success';
      case 'aborted': return 'cds-badge-error';
      case 'failed': return 'cds-badge-error';
      case 'running': return 'cds-badge-info';
      default: return 'cds-badge-neutral';
    }
  };

  return (
    <div className="cds-tile">
      <div className="cds-tile-header">
        <div>
          <h2 className="cds-tile-title">{t('history.title')}</h2>
          <p className="cds-helper-text mb-0">{t('history.desc')}</p>
        </div>
        <button onClick={fetchRuns} disabled={loading} className="cds-btn-secondary cds-btn-sm">
          <Reset size={14} className={loading ? 'animate-spin' : ''} />
          <span>{t('history.refresh_btn')}</span>
        </button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left font-mono text-xs">
          <thead className="bg-[var(--cds-background)] text-[var(--cds-text-helper)] uppercase text-[10px] border-b border-[var(--cds-border-subtle)]">
            <tr>
              <th className="p-3">{t('history.tbl_run_id')}</th>
              <th className="p-3">{t('history.tbl_status')}</th>
              <th className="p-3">{t('history.tbl_total_reqs')}</th>
              <th className="p-3">{t('history.tbl_actual_rps')}</th>
              <th className="p-3">{t('history.tbl_health')}</th>
              <th className="p-3">{t('history.tbl_trend')}</th>
              <th className="p-3">{t('history.tbl_start_time')}</th>
              <th className="p-3">{t('history.tbl_actions')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)]">
            {loading ? (
              <tr><td colSpan={8} className="p-6 text-center text-[var(--cds-text-helper)]">{t('history.loading')}</td></tr>
            ) : runs.length === 0 ? (
              <tr><td colSpan={8} className="p-6 text-center text-[var(--cds-text-helper)]">{t('history.empty')}</td></tr>
            ) : (
              runs.slice().reverse().map(run => {
                const totalReqs = run.summary ? run.summary.total_requests : '-';
                const actualRps = run.summary?.actual_rps ? run.summary.actual_rps.toFixed(1) : '-';
                const score = (run as any).diagnostics ? `${(run as any).diagnostics.health_score}/100` : '-';
                const timeStr = run.start_time ? formatTime(run.start_time) : '-';

                return (
                  <tr key={run.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                    <td className="p-3 font-semibold text-[var(--cds-text-primary)]">{run.id}</td>
                    <td className="p-3">
                      <span className={getStatusBadge(run.status)}>{run.status}</span>
                    </td>
                    <td className="p-3">{totalReqs}</td>
                    <td className="p-3 text-[var(--cds-link)]">{actualRps}</td>
                    <td className="p-3 text-[var(--cds-support-success)] font-bold">{score}</td>
                    <td className="p-3">
                      {trends[run.id] ? (
                        <span className={trends[run.id].regressed ? 'cds-badge-error' : 'cds-badge-success'}>
                          P99 {trends[run.id].delta_p99_pct >= 0 ? '+' : ''}{trends[run.id].delta_p99_pct.toFixed(0)}%
                        </span>
                      ) : (
                        <button onClick={() => handleCheckTrend(run.id)} className="cds-btn-ghost cds-btn-sm">
                          <Analytics size={12} /> {t('history.check_trend')}
                        </button>
                      )}
                    </td>
                    <td className="p-3 text-[var(--cds-text-helper)]">{timeStr}</td>
                    <td className="p-3">
                      <div className="flex items-center gap-1">
                        <button onClick={() => onViewCockpit(run.id)} className="cds-btn-secondary cds-btn-sm px-1.5" title={t('history.view_cockpit', 'Live Cockpit')}>
                          <Activity size={14} />
                        </button>
                        <button onClick={() => onInspectRun(run.id)} className="cds-btn-secondary cds-btn-sm px-1.5" title={t('history.inspect_telemetry')}>
                          <Document size={14} />
                        </button>
                        <button onClick={() => onOpenAdvisory(run.id)} className="cds-btn-ghost cds-btn-sm px-1.5" title={t('history.remediation_advisory')}>
                          <Flash size={14} className="text-[var(--cds-support-warning)]" />
                        </button>
                        <button
                          onClick={() => handleShare(run.id)}
                          title={t('history.share_public')}
                          className="cds-btn-ghost cds-btn-sm px-1.5"
                        >
                          <Share size={14} />
                        </button>
                        <button
                          onClick={() => handleCopyBadge(run.id)}
                          title={t('history.copy_badge')}
                          className="cds-btn-ghost cds-btn-sm px-1.5"
                        >
                          <BadgeIcon size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
