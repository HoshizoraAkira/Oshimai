import React, { useState } from 'react';
import { Analytics } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { opsService } from '../../services/opsService';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface BenchmarkPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

export default function BenchmarkPanel({ showToast }: BenchmarkPanelProps) {
  const { t } = useTranslation();
  const [category, setCategory] = useState('ecommerce');
  const [healthScore, setHealthScore] = useState<number | string>(90);
  const [p99Ms, setP99Ms] = useState<number | string>(200);
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState<any>(null);

  const runCompare = async () => {
    setBusy(true);
    setReport(null);
    try {
      const data = await opsService.compareBenchmark({
        category,
        health_score: parseInt(String(healthScore)),
        p99_ms: parseFloat(String(p99Ms)),
      });
      setReport(data);
    } catch (err: any) {
      showToast(t('ops.benchmark.toast_failed', 'Benchmark compare failed: %s', [err.message]), 'error');
    } finally {
      setBusy(false);
    }
  };

  return (
    <SectionTile eyebrow={t('ops.benchmark.eyebrow')} title={t('ops.benchmark.title')} hint={t('ops.benchmark.hint')}>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <FormField label={t('ops.benchmark.category')}>
          <select value={category} onChange={e => setCategory(e.target.value)} className="cds-select">
            <option value="ecommerce">{t('ops.benchmark.cat_ecommerce')}</option>
            <option value="fintech">{t('ops.benchmark.cat_fintech')}</option>
            <option value="saas_b2b">{t('ops.benchmark.cat_saas_b2b')}</option>
            <option value="other">{t('ops.benchmark.cat_other')}</option>
          </select>
        </FormField>
        <FormField label={t('ops.benchmark.health_score')}>
          <input type="number" min={0} max={100} value={healthScore} onChange={e => setHealthScore(e.target.value)} className="cds-input" />
        </FormField>
        <FormField label={t('ops.benchmark.p99_latency')}>
          <input type="number" min={0} value={p99Ms} onChange={e => setP99Ms(e.target.value)} className="cds-input" />
        </FormField>
      </div>
      <button onClick={runCompare} disabled={busy} className="cds-btn-primary cds-btn-sm normal-case">
        <Analytics size={14} />
        <span>{busy ? t('ops.benchmark.btn_busy') : t('ops.benchmark.btn_compare')}</span>
      </button>
      {report && (
        report.sample_count < 3 ? (
          <div className="cds-notification-warning text-xs">
            <span>{report.message || t('ops.benchmark.not_enough_samples')}</span>
          </div>
        ) : (
          <div className="border border-[var(--cds-border-subtle)] p-4 space-y-3 font-mono text-xs">
            <div className="text-[var(--cds-text-helper)]">{t('ops.benchmark.compared_against', 'Compared against %s other runs in category "%s"', [report.sample_count, category])}</div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <div className="text-[10px] text-[var(--cds-text-helper)] uppercase">{t('ops.benchmark.health_percentile')}</div>
                <div className="text-2xl font-bold text-[var(--cds-support-success)]">{report.health_score_percentile?.toFixed(0)}%</div>
              </div>
              <div>
                <div className="text-[10px] text-[var(--cds-text-helper)] uppercase">{t('ops.benchmark.latency_percentile')}</div>
                <div className="text-2xl font-bold text-[var(--cds-interactive)]">{report.latency_percentile?.toFixed(0)}%</div>
              </div>
            </div>
            <p className="text-[var(--cds-text-secondary)] font-sans">{report.message}</p>
          </div>
        )
      )}
    </SectionTile>
  );
}
