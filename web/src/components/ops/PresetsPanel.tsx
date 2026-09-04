import React, { useState, useEffect } from 'react';
import { Reset, TrashCan, Edit, Flag } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { presetsService } from '../../services/presetsService';
import { CulturalPreset, ToastType } from '../../types/models';
import SectionTile from '../common/SectionTile';
import FormField from '../common/FormField';

export interface PresetsPanelProps {
  showToast: (msg: string, type?: ToastType) => void;
}

const emptyForm = { name: '', descriptionId: '', descriptionEn: '', peakMultiplier: 10, durationSec: 180 };

export default function PresetsPanel({ showToast }: PresetsPanelProps) {
  const { t, lang } = useTranslation();

  const [presetsList, setPresetsList] = useState<CulturalPreset[]>([]);
  const [loading, setLoading] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchPresets = async () => {
    setLoading(true);
    try {
      const data = await presetsService.getCulturalPresets();
      setPresetsList(data || []);
    } catch (err: any) {
      showToast(t('ops.presets.toast_fetch_err', 'Failed to load presets: %s', [err.message]), 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPresets();
  }, []);

  const startEdit = (p: CulturalPreset) => {
    setEditingId(p.id);
    setForm({
      name: p.name,
      descriptionId: p.description_id || '',
      descriptionEn: p.description_en || '',
      peakMultiplier: p.peak_multiplier,
      durationSec: p.suggested_total_duration_sec,
    });
  };

  const cancelEdit = () => {
    setEditingId(null);
    setForm(emptyForm);
  };

  const submit = async () => {
    if (!form.name.trim() || !form.peakMultiplier || !form.durationSec) {
      showToast(t('ops.presets.toast_fields_req', 'Name, peak multiplier, and duration are required.'), 'error');
      return;
    }
    const payload = {
      name: form.name,
      description_id: form.descriptionId,
      description_en: form.descriptionEn,
      peak_multiplier: parseInt(String(form.peakMultiplier)) || 0,
      suggested_total_duration_sec: parseInt(String(form.durationSec)) || 0,
    };
    setSaving(true);
    try {
      if (editingId) {
        const updated = await presetsService.updateCulturalPreset(editingId, payload);
        showToast(t('ops.presets.toast_update_ok', 'Preset "%s" updated.', [updated.name]), 'success');
      } else {
        const created = await presetsService.createCulturalPreset(payload);
        showToast(t('ops.presets.toast_create_ok', 'Preset "%s" created.', [created.name]), 'success');
      }
      cancelEdit();
      fetchPresets();
    } catch (err: any) {
      const key = editingId ? 'ops.presets.toast_update_err' : 'ops.presets.toast_create_err';
      const fallback = editingId ? 'Failed to update preset: %s' : 'Failed to create preset: %s';
      showToast(t(key, fallback, [err.message]), 'error');
    } finally {
      setSaving(false);
    }
  };

  const deletePreset = async (p: CulturalPreset) => {
    if (!confirm(t('ops.presets.confirm_delete', 'Delete preset %s?', [p.name]))) return;
    try {
      await presetsService.deleteCulturalPreset(p.id);
      showToast(t('ops.presets.toast_delete_ok', 'Preset "%s" deleted.', [p.name]), 'success');
      if (editingId === p.id) cancelEdit();
      fetchPresets();
    } catch (err: any) {
      showToast(t('ops.presets.toast_delete_err', 'Failed to delete preset: %s', [err.message]), 'error');
    }
  };

  const describe = (p: CulturalPreset) =>
    lang === 'id' ? (p.description_id || p.description_en) : (p.description_en || p.description_id);

  return (
    <div className="space-y-6">
      <div className="cds-tile">
        <div className="cds-tile-header">
          <div>
            <div className="cds-tile-eyebrow">{t('ops.presets.eyebrow', 'Load Shape')}</div>
            <h3 className="cds-tile-title">
              {t('ops.presets.title', 'Cultural Presets')}
              <span className="normal-case font-normal text-[var(--cds-text-helper)] ml-1.5">({t('ops.presets.hint', '%s preset(s)', [presetsList.length])})</span>
            </h3>
          </div>
          <button onClick={fetchPresets} disabled={loading} className="cds-btn-secondary cds-btn-sm">
            <Reset size={14} className={loading ? 'animate-spin' : ''} />
            <span>{t('common.refresh')}</span>
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-[var(--cds-background)] text-[var(--cds-text-helper)] uppercase text-[10px] border-b border-[var(--cds-border-subtle)]">
              <tr>
                <th className="p-3">{t('ops.presets.tbl_name', 'Name')}</th>
                <th className="p-3">{t('ops.presets.tbl_type', 'Type')}</th>
                <th className="p-3">{t('ops.presets.tbl_peak', 'Peak')}</th>
                <th className="p-3">{t('ops.presets.tbl_duration', 'Duration')}</th>
                <th className="p-3">{t('ops.presets.tbl_actions', 'Actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)]">
              {presetsList.map(p => (
                <tr key={p.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                  <td className="p-3 text-[var(--cds-text-primary)] font-semibold">
                    {p.name}
                    {describe(p) && <div className="text-[10px] font-normal text-[var(--cds-text-helper)] normal-case mt-0.5 max-w-xs">{describe(p)}</div>}
                  </td>
                  <td className="p-3">
                    <span className={p.is_custom ? 'cds-badge-novel' : 'cds-badge-warning'}>
                      {p.is_custom ? t('ops.presets.badge_custom', 'CUSTOM') : t('ops.presets.badge_builtin', 'BUILT-IN')}
                    </span>
                  </td>
                  <td className="p-3">{p.peak_multiplier}x</td>
                  <td className="p-3">{p.suggested_total_duration_sec}s</td>
                  <td className="p-3">
                    {p.is_custom ? (
                      <div className="flex items-center gap-2">
                        <button onClick={() => startEdit(p)} className="cds-btn-ghost cds-btn-sm">
                          <Edit size={12} /> {t('ops.presets.btn_edit', 'Edit')}
                        </button>
                        <button onClick={() => deletePreset(p)} className="cds-btn-ghost cds-btn-sm px-1.5" title={t('ops.presets.btn_delete', 'Delete')}>
                          <TrashCan size={14} className="text-[var(--cds-support-error)]" />
                        </button>
                      </div>
                    ) : (
                      <span className="text-[var(--cds-text-helper)] normal-case" title={t('ops.presets.toast_builtin_readonly', 'Built-in presets cannot be edited or deleted.')}>
                        {t('ops.presets.toast_builtin_readonly', 'Built-in presets cannot be edited or deleted.')}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <SectionTile
        eyebrow={editingId ? t('ops.presets.edit_eyebrow', 'Edit') : t('ops.presets.create_eyebrow', 'Create')}
        title={editingId ? t('ops.presets.edit_title', 'Edit Preset') : t('ops.presets.create_title', 'New Preset')}
      >
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <FormField label={t('ops.presets.field_name', 'Preset Name')} className="md:col-span-3">
            <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="Black Friday Doorbuster" className="cds-input" />
          </FormField>
          <FormField label={t('ops.presets.field_peak', 'Peak Multiplier (x baseline VUs)')}>
            <input type="number" min={1} value={form.peakMultiplier} onChange={e => setForm({ ...form, peakMultiplier: parseInt(e.target.value) || 0 })} className="cds-input" />
          </FormField>
          <FormField label={t('ops.presets.field_duration', 'Suggested Duration (seconds)')}>
            <input type="number" min={1} value={form.durationSec} onChange={e => setForm({ ...form, durationSec: parseInt(e.target.value) || 0 })} className="cds-input" />
          </FormField>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormField label={t('ops.presets.field_desc_id', 'Description (Bahasa Indonesia)')}>
            <textarea value={form.descriptionId} onChange={e => setForm({ ...form, descriptionId: e.target.value })} rows={2} className="cds-textarea" />
          </FormField>
          <FormField label={t('ops.presets.field_desc_en', 'Description (English)')}>
            <textarea value={form.descriptionEn} onChange={e => setForm({ ...form, descriptionEn: e.target.value })} rows={2} className="cds-textarea" />
          </FormField>
        </div>
        <p className="cds-helper-text">{t('ops.presets.hint_shape', 'Custom presets use a generic ramp/hold/taper shape by default. A full custom ramping curve can be set via the API/CLI ("shape" field).')}</p>
        <div className="flex items-center gap-2">
          <button onClick={submit} disabled={saving} className="cds-btn-primary cds-btn-sm normal-case">
            <Flag size={14} />
            <span>{editingId ? (saving ? t('ops.presets.btn_updating', 'Saving...') : t('ops.presets.btn_update', 'Save Changes')) : (saving ? t('ops.presets.btn_creating', 'Creating...') : t('ops.presets.btn_create', 'Create Preset'))}</span>
          </button>
          {editingId && (
            <button onClick={cancelEdit} className="cds-btn-secondary cds-btn-sm normal-case">
              {t('ops.presets.btn_cancel_edit', 'Cancel')}
            </button>
          )}
        </div>
      </SectionTile>
    </div>
  );
}
