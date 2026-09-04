import React, { useState, useEffect } from 'react';
import { Reset, PlayFilled } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { opsService } from '../../services/opsService';
import { secToNs } from '../../utils/units';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface AgentsPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

export default function AgentsPanel({ showToast }: AgentsPanelProps) {
  const { t, formatTime } = useTranslation();
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [scenarioYaml, setScenarioYaml] = useState('');
  const [regions, setRegions] = useState('');
  const [vus, setVus] = useState<number | string>(10);
  const [durationSec, setDurationSec] = useState<number | string>(15);
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<any>(null);

  const fetchAgents = async () => {
    setLoading(true);
    try {
      const data = await opsService.listAgents();
      setAgents(data || []);
    } catch (err: any) {
      showToast(t('ops.agents.toast_fetch_err', 'Failed to load agents: %s', [err.message]), 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAgents();
  }, []);

  const runMultiRegion = async () => {
    if (!scenarioYaml.trim()) {
      showToast(t('ops.agents.toast_yaml_req', 'Scenario YAML is required for multi-region run.'), 'error');
      return;
    }
    setRunning(true);
    setResult(null);
    try {
      const data = await (opsService as any).startMultiRegionRun?.({
        base: {
          scenario_yaml: scenarioYaml,
          load_config: {
            profile: 'flat_vu',
            vus: Number(vus) || 5,
            duration: secToNs(Number(durationSec) || 15),
          },
        },
        regions: regions.trim() ? regions.split(',').map(r => r.trim()).filter(Boolean) : undefined,
      });
      setResult(data);
      showToast(t('ops.agents.toast_run_done', 'Multi-region run completed across %s agent(s).', [data?.agents_used || 1]), 'success');
    } catch (err: any) {
      showToast(t('ops.agents.toast_run_err', 'Multi-region run failed: %s', [err.message]), 'error');
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="cds-tile">
        <div className="cds-tile-header">
          <div>
            <div className="cds-tile-eyebrow">{t('ops.agents.eyebrow')}</div>
            <h3 className="cds-tile-title">
              {t('ops.agents.title')}
              <span className="normal-case font-normal text-[var(--cds-text-helper)] ml-1.5">({t('ops.agents.hint', '%s agent(s)', [agents.length])})</span>
            </h3>
          </div>
          <button onClick={fetchAgents} disabled={loading} className="cds-btn-secondary cds-btn-sm">
            <Reset size={14} className={loading ? 'animate-spin' : ''} />
            <span>{t('common.refresh')}</span>
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-[var(--cds-background)] text-[var(--cds-text-helper)] uppercase text-[10px] border-b border-[var(--cds-border-subtle)]">
              <tr>
                <th className="p-3">{t('ops.agents.agent_id')}</th>
                <th className="p-3">{t('ops.agents.region')}</th>
                <th className="p-3">{t('ops.agents.registered')}</th>
                <th className="p-3">{t('ops.agents.last_seen')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)]">
              {agents.length === 0 ? (
                <tr><td colSpan={4} className="p-4 text-center text-[var(--cds-text-helper)]">{t('ops.agents.empty')}</td></tr>
              ) : agents.map(a => (
                <tr key={a.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                  <td className="p-3 text-[var(--cds-text-primary)] font-semibold">{a.id}</td>
                  <td className="p-3"><span className="cds-badge-info">{a.region || 'default'}</span></td>
                  <td className="p-3">{a.registered_at ? formatTime(a.registered_at) : '-'}</td>
                  <td className="p-3">{a.last_seen ? formatTime(a.last_seen) : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <SectionTile eyebrow={t('ops.agents.load_eyebrow')} title={t('ops.agents.load_title')}>
        <FormField label={t('ops.agents.scenario_yaml')}>
          <textarea
            value={scenarioYaml}
            onChange={e => setScenarioYaml(e.target.value)}
            rows={6}
            placeholder={'id: multi_region_smoke\nbase_url: https://api.example.com\ninitial_step_id: ping\nsteps:\n  ping:\n    id: ping\n    request:\n      path: /ping'}
            className="cds-textarea"
          />
        </FormField>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <FormField label={t('ops.agents.regions_label')} helper={t('ops.agents.regions_helper')}>
            <input type="text" value={regions} onChange={e => setRegions(e.target.value)} placeholder="jakarta, singapore, sydney" className="cds-input" />
          </FormField>
          <FormField label={t('ops.agents.vus_per_agent')}>
            <input type="number" min={1} value={vus} onChange={e => setVus(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.agents.duration_sec')}>
            <input type="number" min={1} value={durationSec} onChange={e => setDurationSec(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <button onClick={runMultiRegion} disabled={running} className="cds-btn-primary cds-btn-sm normal-case">
          <PlayFilled size={14} />
          <span>{running ? t('ops.agents.btn_running') : t('ops.agents.btn_run')}</span>
        </button>

        {result && (
          <div className="border border-[var(--cds-border-subtle)] p-4 space-y-3 font-mono text-xs">
            <div className="flex items-center gap-2">
              <span className="cds-badge-success">{t('ops.agents.badge_complete')}</span>
              <span className="text-[var(--cds-text-secondary)]">{t('ops.agents.result_summary', '%s requests across %s agents', [result.total_requests, result.agents_used])}</span>
            </div>
            {result.regional_results && (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {Object.entries(result.regional_results).map(([reg, r]: [string, any]) => (
                  <div key={reg} className="bg-[var(--cds-background)] p-3 border border-[var(--cds-border-subtle)] space-y-1">
                    <div className="text-[var(--cds-text-primary)] font-bold">{reg}</div>
                    <div className="text-[var(--cds-text-secondary)]">{t('ops.agents.region_stat', '%s req · %s err · P99 %sms', [r.requests, r.errors, r.p99_ms?.toFixed(1) || '-'])}</div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </SectionTile>
    </div>
  );
}
