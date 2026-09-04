import React, { useEffect, useState } from 'react';
import { Activity, CheckmarkFilled, WarningAltFilled } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { streamService } from '../services/streamService';
import { Run } from '../types/models';

export interface PublicStatusPageProps {
  token: string;
}

export default function PublicStatusPage({ token }: PublicStatusPageProps) {
  const { t } = useTranslation();
  const [run, setRun] = useState<Run | null>(null);
  const [live, setLive] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    runsService.getPublicRun(token)
      .then(setRun)
      .catch(e => setError(e.message));

    const subscription = streamService.subscribePublicRunStream(
      token,
      (data) => setLive(data),
      () => {}
    );

    return () => subscription.close();
  }, [token]);

  if (error) {
    return (
      <div className="min-h-screen bg-[var(--cds-background)] text-[var(--cds-text-primary)] flex items-center justify-center font-mono text-sm">
        <div className="p-6 bg-[var(--cds-layer-01)] border border-[var(--cds-support-error)] text-[var(--cds-support-error-text)]">{error}</div>
      </div>
    );
  }

  if (!run) {
    return (
      <div className="min-h-screen bg-[var(--cds-background)] text-[var(--cds-text-helper)] flex items-center justify-center font-mono text-sm">
        {t('public_status.loading')}
      </div>
    );
  }

  const score = (run as any).diagnostics?.health_score;
  const isHealthy = score != null && score >= 85;

  return (
    <div className="min-h-screen bg-[var(--cds-background)] text-[var(--cds-text-primary)] font-mono flex flex-col items-center py-16 px-4">
      <div className="w-full max-w-2xl space-y-6">
        <div className="text-center space-y-1">
          <div className="text-[11px] text-[var(--cds-interactive)] uppercase tracking-widest">{t('public_status.title')}</div>
          <h1 className="text-lg font-semibold text-white">{run.id}</h1>
          <span className="inline-flex items-center gap-1.5 text-xs px-2 py-1 bg-[var(--cds-interactive)]/20 text-[var(--cds-link)] border border-[var(--cds-interactive)]">
            <Activity size={14} className={run.status === 'running' ? 'animate-pulse' : ''} />
            {(live?.status || run.status || '').toUpperCase()}
          </span>
        </div>

        <div className="grid grid-cols-3 gap-3 text-center">
          <div className="p-4 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)]">
            <div className="text-[10px] text-[var(--cds-text-helper)] uppercase">{t('public_status.current_rps')}</div>
            <div className="text-xl font-bold text-[var(--cds-interactive)]">{live?.current_rps?.toFixed(1) ?? run.summary?.actual_rps?.toFixed(1) ?? '-'}</div>
          </div>
          <div className="p-4 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)]">
            <div className="text-[10px] text-[var(--cds-text-helper)] uppercase">{t('public_status.error_rate')}</div>
            <div className="text-xl font-bold text-[var(--cds-support-warning)]">{((live?.error_rate ?? 0) * 100).toFixed(1)}%</div>
          </div>
          <div className="p-4 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)]">
            <div className="text-[10px] text-[var(--cds-text-helper)] uppercase">{t('public_status.health_score')}</div>
            <div className={`text-xl font-bold ${isHealthy ? 'text-[var(--cds-support-success)]' : 'text-[var(--cds-support-error-text)]'}`}>{score != null ? `${score}/100` : '-'}</div>
          </div>
        </div>

        {(run as any).diagnostics && (
          <div className="p-4 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] flex items-start gap-3">
            {isHealthy ? <CheckmarkFilled size={20} className="text-[var(--cds-support-success)] shrink-0 mt-0.5" /> : <WarningAltFilled size={20} className="text-[var(--cds-support-warning)] shrink-0 mt-0.5" />}
            <div className="text-xs text-[var(--cds-text-secondary)] leading-relaxed font-sans">
              <div className="text-white font-semibold mb-1">{(run as any).diagnostics.status_label}</div>
              {(run as any).diagnostics.summary}
            </div>
          </div>
        )}

        <p className="text-center text-[10px] text-[var(--cds-border-strong)]">{t('public_status.disclaimer')}</p>
      </div>
    </div>
  );
}
