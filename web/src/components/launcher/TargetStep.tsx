import React from 'react';
import { Globe, ChevronRight } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { ToastType, TargetVerifyStatus } from '../../types/models';
import TargetVerificationPanel from '../TargetVerificationPanel';

export interface TargetStepProps {
  targetBaseUrl: string;
  setTargetBaseUrl: (url: string) => void;
  environment: string;
  setEnvironment: (env: string) => void;
  avgTxnValue: number | string;
  setAvgTxnValue: (val: any) => void;
  txnPerMinute: number | string;
  setTxnPerMinute: (val: any) => void;
  benchmarkCategory: string;
  setBenchmarkCategory: (cat: string) => void;
  verifyStatus: TargetVerifyStatus | null;
  onVerifyStatusChange: (status: TargetVerifyStatus | null) => void;
  onNext: () => void;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function TargetStep({
  targetBaseUrl,
  setTargetBaseUrl,
  environment,
  setEnvironment,
  avgTxnValue,
  setAvgTxnValue,
  txnPerMinute,
  setTxnPerMinute,
  benchmarkCategory,
  setBenchmarkCategory,
  verifyStatus,
  onVerifyStatusChange,
  onNext,
  showToast,
}: TargetStepProps) {
  const { t } = useTranslation();
  const needsVerification = !!verifyStatus && !verifyStatus.is_private && !verifyStatus.is_verified;

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Globe size={16} className="text-[var(--cds-interactive)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('launcher.step1_heading')}</h2>
        </div>
        <span className="text-[var(--cds-text-helper)]">{t('launcher.step_counter', 'Step %s of 5', [1])}</span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="md:col-span-2">
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.target_base_url', 'Target Base URL')}
          </label>
          <input
            type="text"
            value={targetBaseUrl}
            onChange={e => setTargetBaseUrl(e.target.value)}
            placeholder="https://api.yourcompany.com"
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs font-mono"
          />
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.environment', 'Environment')}
          </label>
          <select
            value={environment}
            onChange={e => setEnvironment(e.target.value)}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          >
            <option value="">{t('launcher.env_default', 'Default / Staging')}</option>
            <option value="staging">{t('launcher.env_staging', 'Staging')}</option>
            <option value="production">{t('launcher.env_production', 'Production (Requires 2nd Approver)')}</option>
          </select>
        </div>
      </div>

      <TargetVerificationPanel targetBaseUrl={targetBaseUrl} showToast={showToast} onStatusChange={onVerifyStatusChange} />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2 border-t border-[var(--cds-border-subtle)]">
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.avg_txn_value', 'Avg Transaction Value (Rp)')}
          </label>
          <input
            type="number"
            value={avgTxnValue}
            onChange={e => setAvgTxnValue(e.target.value)}
            placeholder="150000"
            min={0}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          />
          <span className="text-[10px] text-[var(--cds-text-helper)] mt-1 block">
            {t('launcher.avg_txn_hint', 'Used to estimate financial loss from error rates.')}
          </span>
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.est_txn_min', 'Estimated Txn / Minute')}
          </label>
          <input
            type="number"
            value={txnPerMinute}
            onChange={e => setTxnPerMinute(e.target.value)}
            placeholder="40"
            min={0}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          />
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.benchmark_category', 'Industry Benchmark')}
          </label>
          <select
            value={benchmarkCategory}
            onChange={e => setBenchmarkCategory(e.target.value)}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          >
            <option value="">{t('launcher.benchmark_none')}</option>
            <option value="ecommerce">{t('launcher.benchmark_ecommerce')}</option>
            <option value="fintech">{t('launcher.benchmark_fintech')}</option>
            <option value="saas_b2b">{t('launcher.benchmark_saas_b2b')}</option>
            <option value="other">{t('launcher.benchmark_other')}</option>
          </select>
        </div>
      </div>

      <div className="flex items-center justify-end gap-3 pt-3 border-t border-[var(--cds-border-subtle)]">
        {needsVerification && (
          <span className="text-[10px] text-[var(--cds-support-warning)]">
            {t('launcher.verification_required_hint', 'Verify target ownership before continuing.')}
          </span>
        )}
        <button
          type="button"
          onClick={onNext}
          disabled={needsVerification}
          className="px-4 py-2 bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold flex items-center gap-1.5"
        >
          <span>{t('launcher.next_step', 'Next Step')}: {t('launcher.step_2', 'Workload Profile')}</span>
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}
