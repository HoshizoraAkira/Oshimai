import React, { useState, useEffect } from 'react';
import { Reset, Renew, TrashCan, Time } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { opsService } from '../../services/opsService';
import { secToNs } from '../../utils/units';
import { ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface GameDayPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

export default function GameDayPanel({ showToast }: GameDayPanelProps) {
  const { t, formatDateTime } = useTranslation();
  const rawDays = t('common.days');
  const WEEKDAYS = Array.isArray(rawDays) ? rawDays : ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

  const [schedules, setSchedules] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [name, setName] = useState('');
  const [weekday, setWeekday] = useState<number | string>(2);
  const [hourUtc, setHourUtc] = useState<number | string>(9);
  const [minuteUtc, setMinuteUtc] = useState<number | string>(0);
  const [scenarioYaml, setScenarioYaml] = useState('');
  const [vus, setVus] = useState<number | string>(20);
  const [durationSec, setDurationSec] = useState<number | string>(300);
  const [creating, setCreating] = useState(false);

  const fetchSchedules = async () => {
    setLoading(true);
    try {
      const data = await opsService.listGameDaySchedules();
      setSchedules(data || []);
    } catch (err: any) {
      showToast(t('ops.gameday.toast_fetch_err', 'Failed to load schedules: %s', [err.message]), 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSchedules();
  }, []);

  const createSchedule = async () => {
    if (!name.trim() || !scenarioYaml.trim()) {
      showToast(t('ops.gameday.toast_fields_req'), 'error');
      return;
    }
    setCreating(true);
    try {
      const data = await opsService.createGameDaySchedule({
        name,
        weekday: parseInt(String(weekday)),
        hour_utc: parseInt(String(hourUtc)),
        minute_utc: parseInt(String(minuteUtc)),
        run: {
          scenario_yaml: scenarioYaml,
          load_config: {
            profile: 'flat_vu',
            vus: parseInt(String(vus)) || 5,
            duration: secToNs(parseInt(String(durationSec)) || 60),
          },
        },
      } as any);
      showToast(t('ops.gameday.toast_create_ok', 'GameDay schedule "%s" created (%s).', [name, (data as any).id]), 'success');
      setName('');
      setScenarioYaml('');
      fetchSchedules();
    } catch (err: any) {
      showToast(t('ops.gameday.toast_create_err', 'Failed to create schedule: %s', [err.message]), 'error');
    } finally {
      setCreating(false);
    }
  };

  const toggleSchedule = async (id: string, enabled: boolean) => {
    try {
      await opsService.toggleGameDaySchedule(id, enabled);
      fetchSchedules();
    } catch (err: any) {
      showToast(t('ops.gameday.toast_toggle_err', 'Failed to toggle schedule: %s', [err.message]), 'error');
    }
  };

  const deleteSchedule = async (id: string) => {
    if (!confirm(t('ops.gameday.confirm_delete', 'Delete schedule %s?', [id]))) return;
    try {
      await opsService.deleteGameDaySchedule(id);
      fetchSchedules();
    } catch (err: any) {
      showToast(t('ops.gameday.toast_delete_err', 'Failed to delete schedule: %s', [err.message]), 'error');
    }
  };

  return (
    <div className="space-y-6">
      <div className="cds-tile">
        <div className="cds-tile-header">
          <div>
            <div className="cds-tile-eyebrow">{t('ops.gameday.eyebrow')}</div>
            <h3 className="cds-tile-title">
              {t('ops.gameday.title')}
              <span className="normal-case font-normal text-[var(--cds-text-helper)] ml-1.5">({t('ops.gameday.hint', '%s schedule(s)', [schedules.length])})</span>
            </h3>
          </div>
          <button onClick={fetchSchedules} disabled={loading} className="cds-btn-secondary cds-btn-sm">
            <Reset size={14} className={loading ? 'animate-spin' : ''} />
            <span>{t('common.refresh')}</span>
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-[var(--cds-background)] text-[var(--cds-text-helper)] uppercase text-[10px] border-b border-[var(--cds-border-subtle)]">
              <tr>
                <th className="p-3">{t('ops.gameday.tbl_name')}</th>
                <th className="p-3">{t('ops.gameday.tbl_schedule')}</th>
                <th className="p-3">{t('ops.gameday.tbl_status')}</th>
                <th className="p-3">{t('ops.gameday.tbl_last_run')}</th>
                <th className="p-3">{t('ops.gameday.tbl_actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)]">
              {schedules.length === 0 ? (
                <tr><td colSpan={5} className="p-4 text-center text-[var(--cds-text-helper)]">{t('ops.gameday.empty')}</td></tr>
              ) : schedules.map(s => (
                <tr key={s.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                  <td className="p-3 text-[var(--cds-text-primary)] font-semibold">{s.name}</td>
                  <td className="p-3">{WEEKDAYS[s.weekday % 7]} {String(s.hour_utc).padStart(2, '0')}:{String(s.minute_utc).padStart(2, '0')}</td>
                  <td className="p-3"><span className={s.enabled ? 'cds-badge-success' : 'cds-badge-neutral'}>{s.enabled ? t('common.enabled') : t('common.disabled')}</span></td>
                  <td className="p-3">{s.last_fired_at ? formatDateTime(s.last_fired_at) : t('common.never')}</td>
                  <td className="p-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => toggleSchedule(s.id, !s.enabled)} className="cds-btn-ghost cds-btn-sm">
                        <Renew size={12} /> {s.enabled ? t('common.disable') : t('common.enable')}
                      </button>
                      <button onClick={() => deleteSchedule(s.id)} className="cds-btn-ghost cds-btn-sm px-1.5" title={t('common.delete')}>
                        <TrashCan size={14} className="text-[var(--cds-support-error)]" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <SectionTile eyebrow={t('ops.gameday.create_eyebrow')} title={t('ops.gameday.create_title')}>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <FormField label={t('ops.gameday.field_name')}>
            <input type="text" value={name} onChange={e => setName(e.target.value)} placeholder="checkout-quarterly-gameday" className="cds-input" />
          </FormField>
          <FormField label={t('ops.gameday.field_day')}>
            <select value={weekday} onChange={e => setWeekday(e.target.value)} className="cds-select">
              {WEEKDAYS.map((d, i) => <option key={i} value={i}>{d}</option>)}
            </select>
          </FormField>
          <FormField label={t('ops.gameday.field_hour')}>
            <input type="number" min={0} max={23} value={hourUtc} onChange={e => setHourUtc(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.gameday.field_minute')}>
            <input type="number" min={0} max={59} value={minuteUtc} onChange={e => setMinuteUtc(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <FormField label={t('ops.gameday.field_yaml')}>
          <textarea
            value={scenarioYaml}
            onChange={e => setScenarioYaml(e.target.value)}
            rows={6}
            placeholder={'id: checkout_gameday\nbase_url: https://checkout.example.com\ninitial_step_id: ping\nsteps:\n  ping:\n    id: ping\n    request:\n      path: /ping'}
            className="cds-textarea"
          />
        </FormField>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormField label={t('ops.gameday.field_vus')}>
            <input type="number" min={1} value={vus} onChange={e => setVus(e.target.value)} className="cds-input" />
          </FormField>
          <FormField label={t('ops.gameday.field_duration')}>
            <input type="number" min={1} value={durationSec} onChange={e => setDurationSec(e.target.value)} className="cds-input" />
          </FormField>
        </div>
        <button onClick={createSchedule} disabled={creating} className="cds-btn-primary cds-btn-sm normal-case">
          <Time size={14} />
          <span>{creating ? t('ops.gameday.btn_creating') : t('ops.gameday.btn_create')}</span>
        </button>
      </SectionTile>
    </div>
  );
}
