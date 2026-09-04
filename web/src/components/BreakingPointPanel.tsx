import React, { useState } from 'react';
import { Search, Flash, CheckmarkFilled, WarningAltFilled } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { breakingPointService } from '../services/breakingPointService';
import { ToastType } from '../types/models';

export interface BreakingPointPanelProps {
  scenarioText: string;
  targetBaseUrl: string;
  vus: number | string;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function BreakingPointPanel({ scenarioText, targetBaseUrl, vus, showToast }: BreakingPointPanelProps) {
  const { t } = useTranslation();
  const [running, setRunning] = useState<'autopilot' | 'autofuzz' | null>(null);
  const [result, setResult] = useState<any>(null);
  const [resultType, setResultType] = useState<'autopilot' | 'autofuzz' | null>(null);

  const buildBase = () => {
    const base: any = {
      target_base_url: targetBaseUrl,
      load_config: { profile: 'flat_vu', vus: parseInt(String(vus)) || 10 },
    };
    if (scenarioText.trim().startsWith('{')) {
      base.scenario_json = scenarioText;
    } else {
      base.scenario_yaml = scenarioText;
    }
    return base;
  };

  const runSearch = async (kind: 'autopilot' | 'autofuzz') => {
    if (!scenarioText.trim()) {
      showToast(t('toasts.fill_scenario_first', 'Fill in scenario in step 05 before searching.'), 'warning');
      return;
    }
    setRunning(kind);
    setResult(null);
    try {
      const payload: any = kind === 'autopilot'
        ? { base: buildBase(), min_vus: 1, max_vus: 500, trial_duration_seconds: 4, health_threshold: 70, max_iterations: 7 }
        : { base: buildBase(), trial_duration_seconds: 4, health_threshold: 60, max_iterations: 6 };

      const data = kind === 'autopilot'
        ? await breakingPointService.runAutoPilot(payload)
        : await breakingPointService.runAutoFuzz(payload);

      setResult(data);
      setResultType(kind);
      showToast(t('toasts.search_done', 'Search complete — see results below.'), 'success');
    } catch (err: any) {
      showToast(t('toasts.search_failed', 'Search failed: %s', [err.message]), 'error');
    } finally {
      setRunning(null);
    }
  };

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4 font-mono text-xs">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Search size={16} className="text-[var(--cds-interactive)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('breaking.title')}</h2>
        </div>
        <span className="text-[var(--cds-text-helper)]">{t('breaking.subtitle')}</span>
      </div>

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => runSearch('autopilot')}
          disabled={running !== null}
          className="px-3 py-2 bg-[var(--cds-interactive)]/20 hover:bg-[var(--cds-interactive)]/30 border border-[var(--cds-interactive)] text-[var(--cds-link)] text-[11px] flex items-center gap-1.5 disabled:opacity-40"
        >
          <Search size={14} />
          <span>{running === 'autopilot' ? t('breaking.btn_autopilot_busy') : t('breaking.btn_autopilot')}</span>
        </button>
        <button
          type="button"
          onClick={() => runSearch('autofuzz')}
          disabled={running !== null}
          className="px-3 py-2 bg-[var(--cds-support-error)]/15 hover:bg-[var(--cds-support-error)]/25 border border-[var(--cds-support-error)] text-[var(--cds-support-error-text)] text-[11px] flex items-center gap-1.5 disabled:opacity-40"
        >
          <Flash size={14} />
          <span>{running === 'autofuzz' ? t('breaking.btn_autofuzz_busy') : t('breaking.btn_autofuzz')}</span>
        </button>
      </div>

      {running && (
        <p className="text-[10px] text-[var(--cds-text-helper)]">{t('breaking.waiting_desc')}</p>
      )}

      {result && (
        <div className="space-y-2 pt-2 border-t border-[var(--cds-border-subtle)]">
          <div className="flex items-start gap-2 p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)]">
            {result.found ? (
              <WarningAltFilled size={16} className="text-[var(--cds-support-warning)] shrink-0 mt-0.5" />
            ) : (
              <CheckmarkFilled size={16} className="text-[var(--cds-support-success)] shrink-0 mt-0.5" />
            )}
            <p className="text-[var(--cds-text-secondary)] font-sans leading-relaxed">{result.verdict}</p>
          </div>

          <div className="overflow-x-auto border border-[var(--cds-border-subtle)]">
            <table className="w-full text-left text-[11px]">
              <thead className="bg-[var(--cds-surface-sunken)] text-[var(--cds-text-helper)] uppercase border-b border-[var(--cds-border-subtle)]">
                <tr>
                  <th className="p-2">{resultType === 'autopilot' ? t('breaking.tbl_vus') : t('breaking.tbl_severity')}</th>
                  <th className="p-2">{t('breaking.tbl_health')}</th>
                  <th className="p-2">{t('breaking.tbl_result')}</th>
                </tr>
              </thead>
              <tbody className="bg-[var(--cds-background)] divide-y divide-[var(--cds-border-subtle)]">
                {(result.trials || []).map((trial: any, idx: number) => (
                  <tr key={idx}>
                    <td className="p-2 text-white">{resultType === 'autopilot' ? trial.vus : `${(trial.severity * 100).toFixed(0)}%`}</td>
                    <td className="p-2 text-[var(--cds-interactive)]">{trial.health_score}</td>
                    <td className="p-2">
                      <span className={`px-1.5 py-0.5 border text-[10px] font-bold ${trial.pass ? 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]' : 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]'}`}>
                        {trial.pass ? t('breaking.status_healthy') : t('breaking.status_failed')}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
