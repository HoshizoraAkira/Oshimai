import React, { useState } from 'react';
import { Kubernetes, ChartLine } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { opsService } from '../../services/opsService';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface K8sPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

export default function K8sPanel({ showToast }: K8sPanelProps) {
  const { t, formatTime } = useTranslation();
  const [apiServer, setApiServer] = useState('');
  const [bearerToken, setBearerToken] = useState('');
  const [namespace, setNamespace] = useState('default');
  const [insecure, setInsecure] = useState(false);

  const [labelSelector, setLabelSelector] = useState('app=checkout');
  const [killPercent, setKillPercent] = useState<number | string>(30);
  const [killDuration, setKillDuration] = useState<number | string>(30);
  const [killBusy, setKillBusy] = useState(false);
  const [killStatus, setKillStatus] = useState<any>(null);

  const [hpaName, setHpaName] = useState('checkout-hpa');
  const [watchDuration, setWatchDuration] = useState<number | string>(60);
  const [pollInterval, setPollInterval] = useState<number | string>(5);
  const [watchBusy, setWatchBusy] = useState(false);
  const [report, setReport] = useState<any>(null);

  const connectionFields = () => ({
    api_server: apiServer,
    bearer_token: bearerToken,
    namespace,
    insecure_skip_verify: insecure,
  });

  const runPodKill = async () => {
    if (!apiServer || !bearerToken) {
      showToast(t('ops.k8s.toast_creds_req'), 'error');
      return;
    }
    setKillBusy(true);
    setKillStatus(null);
    try {
      const data = await opsService.k8sPodKill({
        ...connectionFields(),
        label_selector: labelSelector,
        kill_percent: parseFloat(String(killPercent)),
        duration_seconds: parseInt(String(killDuration)),
      });
      setKillStatus(data);
      showToast(t('ops.k8s.toast_kill_ok'), 'warning');
    } catch (err: any) {
      showToast(t('ops.k8s.toast_kill_err', 'Pod-kill failed: %s', [err.message]), 'error');
    } finally {
      setKillBusy(false);
    }
  };

  const runAutoscalerReport = async () => {
    if (!apiServer || !bearerToken) {
      showToast(t('ops.k8s.toast_creds_req'), 'error');
      return;
    }
    setWatchBusy(true);
    setReport(null);
    try {
      const data = await opsService.k8sAutoscalerReport({
        ...connectionFields(),
        hpa_name: hpaName,
        watch_duration_seconds: parseInt(String(watchDuration)),
        poll_interval_seconds: parseInt(String(pollInterval)),
      });
      setReport(data);
      showToast(t('ops.k8s.toast_hpa_ok'), 'success');
    } catch (err: any) {
      showToast(t('ops.k8s.toast_hpa_err', 'Autoscaler report failed: %s', [err.message]), 'error');
    } finally {
      setWatchBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <SectionTile eyebrow={t('ops.k8s.cluster_eyebrow')} title={t('ops.k8s.cluster_title')} hint={t('ops.k8s.cluster_hint')}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormField label={t('ops.k8s.api_server')}>
            <input type="text" value={apiServer} onChange={e => setApiServer(e.target.value)} placeholder="https://k8s-api.internal:6443" className="cds-input" />
          </FormField>
          <FormField label={t('ops.k8s.namespace')}>
            <input type="text" value={namespace} onChange={e => setNamespace(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <FormField label={t('ops.k8s.bearer_token')}>
          <input type="password" value={bearerToken} onChange={e => setBearerToken(e.target.value)} className="cds-input" />
        </FormField>
        <label className="flex items-center gap-2 text-xs text-[var(--cds-text-secondary)]">
          <input type="checkbox" checked={insecure} onChange={e => setInsecure(e.target.checked)} className="accent-[var(--cds-interactive)]" />
          {t('ops.k8s.insecure_tls')}
        </label>
      </SectionTile>

      <SectionTile eyebrow={t('ops.k8s.chaos_eyebrow')} title={t('ops.k8s.chaos_title')}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <FormField label={t('ops.k8s.label_selector')}>
            <input type="text" value={labelSelector} onChange={e => setLabelSelector(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.k8s.kill_percent')}>
            <input type="number" min={0} max={100} value={killPercent} onChange={e => setKillPercent(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.k8s.duration_sec')}>
            <input type="number" min={1} value={killDuration} onChange={e => setKillDuration(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <button onClick={runPodKill} disabled={killBusy} className="cds-btn-danger cds-btn-sm normal-case">
          <Kubernetes size={14} />
          <span>{killBusy ? t('ops.k8s.btn_kill_busy') : t('ops.k8s.btn_kill')}</span>
        </button>
        {killStatus && (
          <div className="border border-[var(--cds-border-subtle)] p-3 font-mono text-xs space-y-1">
            <div>{t('ops.k8s.state')}: <span className="cds-badge-warning">{killStatus.state}</span></div>
            {killStatus.applied_at && <div className="text-[var(--cds-text-helper)]">{t('ops.k8s.applied_at', 'Applied at %s', [formatTime(killStatus.applied_at)])}</div>}
            {killStatus.expires_at && <div className="text-[var(--cds-text-helper)]">{t('ops.k8s.expires_at', 'Expires at %s', [formatTime(killStatus.expires_at)])}</div>}
          </div>
        )}
      </SectionTile>

      <SectionTile eyebrow={t('ops.k8s.hpa_eyebrow')} title={t('ops.k8s.hpa_title')}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <FormField label={t('ops.k8s.hpa_name')}>
            <input type="text" value={hpaName} onChange={e => setHpaName(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.k8s.watch_duration')}>
            <input type="number" min={1} value={watchDuration} onChange={e => setWatchDuration(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.k8s.poll_interval')}>
            <input type="number" min={1} value={pollInterval} onChange={e => setPollInterval(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <button onClick={runAutoscalerReport} disabled={watchBusy} className="cds-btn-primary cds-btn-sm normal-case">
          <ChartLine size={14} />
          <span>{watchBusy ? t('ops.k8s.btn_hpa_busy') : t('ops.k8s.btn_hpa')}</span>
        </button>
        {report && (
          <div className="border border-[var(--cds-border-subtle)] p-3 space-y-2 font-mono text-xs">
            <div className="flex items-center gap-2">
              <span className={report.scaled_up ? 'cds-badge-success' : 'cds-badge-neutral'}>{report.scaled_up ? t('ops.k8s.scaled_up') : t('ops.k8s.no_scale_up')}</span>
              <span className="text-[var(--cds-text-secondary)]">{t('ops.k8s.replicas_change', '%s → %s replicas', [report.start_replicas, report.peak_replicas])}</span>
            </div>
            <p className="text-[var(--cds-text-secondary)] font-sans">{report.verdict}</p>
          </div>
        )}
      </SectionTile>
    </div>
  );
}
