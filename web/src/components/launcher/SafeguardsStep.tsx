import React from 'react';
import { Security, ChevronLeft, ChevronRight } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';

export interface SafeguardsStepProps {
  cbErrorRate: number | string;
  setCbErrorRate: (val: any) => void;
  cbP99Ms: number | string;
  setCbP99Ms: (val: any) => void;
  onPrev: () => void;
  onNext: () => void;
}

export default function SafeguardsStep({
  cbErrorRate,
  setCbErrorRate,
  cbP99Ms,
  setCbP99Ms,
  onPrev,
  onNext,
}: SafeguardsStepProps) {
  const { t } = useTranslation();

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Security size={16} className="text-[var(--cds-support-warning)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('launcher.step3_heading')}</h2>
        </div>
        <span className="text-[var(--cds-text-helper)]">{t('launcher.step_counter', 'Step %s of 5', [3])}</span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.max_error_rate', 'Max Error Rate Threshold (%)')}
          </label>
          <input 
            type="number" 
            value={cbErrorRate} 
            onChange={e => setCbErrorRate(e.target.value)}
            min={1} 
            max={100}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          />
          <span className="text-[10px] text-[var(--cds-text-helper)] mt-1 block">
            {t('launcher.error_rate_hint', 'Abort run if error rate breaches this limit in sliding window.')}
          </span>
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.max_p99_sla', 'Max P99 Latency SLA (ms)')}
          </label>
          <input 
            type="number" 
            value={cbP99Ms} 
            onChange={e => setCbP99Ms(e.target.value)}
            min={50}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          />
          <span className="text-[10px] text-[var(--cds-text-helper)] mt-1 block">
            {t('launcher.p99_hint', 'Abort run if P99 latency breaches this boundary.')}
          </span>
        </div>
      </div>

      <div className="flex items-center justify-between pt-3 border-t border-[var(--cds-border-subtle)]">
        <button
          type="button"
          onClick={onPrev}
          className="px-3 py-2 bg-[var(--cds-layer-01)] hover:bg-[#333] border border-[var(--cds-border-subtle)] text-white flex items-center gap-1.5"
        >
          <ChevronLeft size={14} />
          <span>{t('launcher.prev_step', 'Previous Step')}</span>
        </button>
        <button
          type="button"
          onClick={onNext}
          className="px-4 py-2 bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] text-white font-semibold flex items-center gap-1.5"
        >
          <span>{t('launcher.next_step', 'Next Step')}: {t('launcher.step_4', 'Chaos Injection')}</span>
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}
