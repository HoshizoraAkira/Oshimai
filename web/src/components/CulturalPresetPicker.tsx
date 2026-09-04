import React, { useState, useEffect } from 'react';
import { Flash } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { presetsService } from '../services/presetsService';
import { CulturalPreset, ToastType } from '../types/models';

export interface CulturalPresetPickerProps {
  onApplyPreset: (preset: CulturalPreset) => void;
  showToast?: (msg: string, type?: ToastType) => void;
}

export default function CulturalPresetPicker({ onApplyPreset, showToast }: CulturalPresetPickerProps) {
  const { t, lang } = useTranslation();
  const [presetsList, setPresetsList] = useState<CulturalPreset[]>([]);
  const [selectedId, setSelectedId] = useState('');

  useEffect(() => {
    presetsService.getCulturalPresets()
      .then(data => setPresetsList(data || []))
      .catch(() => showToast && showToast(t('launcher.toast_load_presets_err', 'Failed to load cultural presets.'), 'error'));
  }, []);

  const selected = presetsList.find(p => p.id === selectedId);
  const desc = selected
    ? (lang === 'id' ? (selected.description_id || selected.description_en) : (selected.description_en || selected.description_id))
    : '';

  return (
    <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-2">
      <div className="flex items-center gap-2">
        <Flash size={14} className="text-[var(--cds-support-warning)]" />
        <span className="text-[11px] text-[var(--cds-support-warning)] font-semibold uppercase">
          {t('launcher.cultural_presets', 'Preset Beban Budaya-Lokal (opsional)')}
        </span>
      </div>
      <select
        value={selectedId}
        onChange={e => setSelectedId(e.target.value)}
        className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
      >
        <option value="">{t('launcher.select_event_scenario', '— Select realistic event scenario —')}</option>
        {presetsList.map(p => (
          <option key={p.id} value={p.id}>{p.is_custom ? `${p.name} (${t('common.custom', 'Custom')})` : p.name}</option>
        ))}
      </select>
      {selected && (
        <div className="space-y-2">
          <p className="text-[11px] text-[var(--cds-text-helper)] leading-relaxed">{desc}</p>
          <button
            type="button"
            onClick={() => onApplyPreset(selected)}
            className="px-2.5 py-1.5 bg-[var(--cds-border-subtle)] hover:bg-[var(--cds-layer-03)] text-white text-[11px] font-semibold"
          >
            {t('launcher.apply_pattern_ramping', 'Apply Load Pattern to Ramping Stages')}
          </button>
        </div>
      )}
    </div>
  );
}
