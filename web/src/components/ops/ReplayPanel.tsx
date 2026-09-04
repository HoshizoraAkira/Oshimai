import React, { useState } from 'react';
import { Search, PlayFilled } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { scenarioService } from '../../services/scenarioService';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface ReplayPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
  onSendToLauncher: (yaml: string, baseUrl?: string) => void;
}

export default function ReplayPanel({ showToast, onSendToLauncher }: ReplayPanelProps) {
  const { t } = useTranslation();
  const [otelTraces, setOtelTraces] = useState('');
  const [traceIds, setTraceIds] = useState<string[] | null>(null);
  const [selectedTraceId, setSelectedTraceId] = useState('');
  const [speedMultiplier, setSpeedMultiplier] = useState<number | string>(1);
  const [baseUrl, setBaseUrl] = useState('');
  const [listing, setListing] = useState(false);
  const [building, setBuilding] = useState(false);

  const listSessions = async () => {
    if (!otelTraces.trim()) {
      showToast(t('ops.replay.toast_json_req'), 'error');
      return;
    }
    setListing(true);
    setTraceIds(null);
    try {
      const data = await scenarioService.listTraceSessions({ otel_traces: otelTraces });
      setTraceIds(data.trace_ids || []);
      if (data.trace_ids?.length) setSelectedTraceId(data.trace_ids[0]);
      showToast(t('ops.replay.toast_sessions_found', 'Found %s replayable session(s).', [data.trace_ids?.length || 0]), 'success');
    } catch (err: any) {
      showToast(t('ops.replay.toast_parse_err', 'Failed to parse traces: %s', [err.message]), 'error');
    } finally {
      setListing(false);
    }
  };

  const buildReplay = async () => {
    if (!selectedTraceId) return;
    setBuilding(true);
    try {
      const data = await scenarioService.buildReplayScenario({
        otel_traces: otelTraces,
        trace_id: selectedTraceId,
        speed_multiplier: parseFloat(String(speedMultiplier)) || 1,
        base_url: baseUrl,
      });
      const yaml = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
      onSendToLauncher(yaml, baseUrl || undefined);
      showToast(t('ops.replay.toast_sent', 'Replay scenario for trace %s sent to Launcher.', [selectedTraceId]), 'success');
    } catch (err: any) {
      showToast(t('ops.replay.toast_build_err', 'Failed to build replay scenario: %s', [err.message]), 'error');
    } finally {
      setBuilding(false);
    }
  };

  return (
    <SectionTile eyebrow={t('ops.replay.eyebrow')} title={t('ops.replay.title')} hint={t('ops.replay.hint')}>
      <FormField label={t('ops.replay.otel_traces')}>
        <textarea
          value={otelTraces}
          onChange={e => setOtelTraces(e.target.value)}
          rows={6}
          placeholder={t('ops.replay.otel_placeholder')}
          className="cds-textarea"
        />
      </FormField>
      <button onClick={listSessions} disabled={listing} className="cds-btn-secondary cds-btn-sm normal-case">
        <Search size={14} />
        <span>{listing ? t('ops.replay.btn_listing') : t('ops.replay.btn_find')}</span>
      </button>

      {traceIds && traceIds.length > 0 && (
        <div className="border-t border-[var(--cds-border-subtle)] pt-4 space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <FormField label={t('ops.replay.field_trace_session')}>
              <select value={selectedTraceId} onChange={e => setSelectedTraceId(e.target.value)} className="cds-select">
                {traceIds.map(id => <option key={id} value={id}>{id}</option>)}
              </select>
            </FormField>
            <FormField label={t('ops.replay.field_multiplier')} helper={t('ops.replay.multiplier_helper')}>
              <input type="number" min={0.1} step={0.1} value={speedMultiplier} onChange={e => setSpeedMultiplier(e.target.value)} className="cds-input" />
            </FormField>
            <FormField label={t('ops.replay.override_url')}>
              <input type="text" value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="https://staging.example.com" className="cds-input" />
            </FormField>
          </div>
          <button onClick={buildReplay} disabled={building} className="cds-btn-primary cds-btn-sm normal-case">
            <PlayFilled size={14} />
            <span>{building ? t('ops.replay.btn_building') : t('ops.replay.btn_send')}</span>
          </button>
        </div>
      )}
      {traceIds && traceIds.length === 0 && (
        <div className="cds-notification-warning text-xs">
          <span>{t('ops.replay.no_sessions')}</span>
        </div>
      )}
    </SectionTile>
  );
}
