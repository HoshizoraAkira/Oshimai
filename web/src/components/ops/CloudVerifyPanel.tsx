import React, { useState } from 'react';
import { Locked } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { verificationService } from '../../services/verificationService';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface CloudVerifyPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

export default function CloudVerifyPanel({ showToast }: CloudVerifyPanelProps) {
  const { t } = useTranslation();
  const [targetUrl, setTargetUrl] = useState('');
  const [accessKey, setAccessKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [region, setRegion] = useState('ap-southeast-1');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<any>(null);

  const runVerify = async () => {
    if (!targetUrl.trim() || !accessKey.trim() || !secretKey.trim()) {
      showToast(t('ops.cloudverify.toast_fields_req'), 'error');
      return;
    }
    setBusy(true);
    setResult(null);
    try {
      const data = await verificationService.confirmCloud({
        target_url: targetUrl,
        aws_access_key: accessKey,
        aws_secret_key: secretKey,
        aws_region: region,
      });
      setResult(data);
      showToast(t('ops.cloudverify.toast_verified', 'Ownership of %s verified via AWS Elastic IP.', [targetUrl]), 'success');
    } catch (err: any) {
      setResult({ verified: false, error: err.message });
    } finally {
      setBusy(false);
    }
  };

  return (
    <SectionTile eyebrow={t('ops.cloudverify.eyebrow')} title={t('ops.cloudverify.title')} hint={t('ops.cloudverify.hint')}>
      <p className="text-xs text-[var(--cds-text-secondary)] leading-relaxed">
        {t('ops.cloudverify.desc')}
      </p>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <FormField label={t('ops.cloudverify.target_url')}>
          <input type="text" value={targetUrl} onChange={e => setTargetUrl(e.target.value)} placeholder="https://api.tokokamu.co.id" className="cds-input" />
        </FormField>
        <FormField label={t('ops.cloudverify.aws_region')}>
          <input type="text" value={region} onChange={e => setRegion(e.target.value)} className="cds-input" />
        </FormField>
        <FormField label={t('ops.cloudverify.aws_access_key')}>
          <input type="text" value={accessKey} onChange={e => setAccessKey(e.target.value)} className="cds-input" />
        </FormField>
        <FormField label={t('ops.cloudverify.aws_secret_key')}>
          <input type="password" value={secretKey} onChange={e => setSecretKey(e.target.value)} className="cds-input" />
        </FormField>
      </div>
      <button onClick={runVerify} disabled={busy} className="cds-btn-primary cds-btn-sm normal-case">
        <Locked size={14} />
        <span>{busy ? t('ops.cloudverify.btn_busy') : t('ops.cloudverify.btn_verify')}</span>
      </button>
      {result && (
        result.verified ? (
          <div className="cds-notification-success text-xs">
            <Locked size={16} className="shrink-0 text-[var(--cds-support-success)]" />
            <span>{t('ops.cloudverify.verified_banner', 'Verified via %s — resource %s (IP %s).', [result.method, result.resource, result.matched_ip])}</span>
          </div>
        ) : (
          <div className="cds-notification-error text-xs">
            <span>{result.error || t('ops.cloudverify.failed_banner')}</span>
          </div>
        )
      )}
    </SectionTile>
  );
}
