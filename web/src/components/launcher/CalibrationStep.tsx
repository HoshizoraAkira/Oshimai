import React from 'react';
import { Activity, ChevronLeft, ChevronRight } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { ToastType } from '../../types/models';
import CulturalPresetPicker from '../CulturalPresetPicker';

export interface RampingStageItem {
  durationSec: number | string;
  targetVus: number | string;
}

export interface CalibrationStepProps {
  profile: string;
  setProfile: (p: string) => void;
  targetRPS: number | string;
  setTargetRPS: (rps: any) => void;
  vus: number | string;
  setVus: (vus: any) => void;
  durationSec: number | string;
  setDurationSec: (sec: any) => void;
  rampingStages: RampingStageItem[];
  onAddStage: () => void;
  onRemoveStage: (idx: number) => void;
  onUpdateStage: (idx: number, field: string, value: any) => void;
  onApplyCulturalPreset: (preset: any) => void;
  onPrev: () => void;
  onNext: () => void;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function CalibrationStep({
  profile,
  setProfile,
  targetRPS,
  setTargetRPS,
  vus,
  setVus,
  durationSec,
  setDurationSec,
  rampingStages,
  onAddStage,
  onRemoveStage,
  onUpdateStage,
  onApplyCulturalPreset,
  onPrev,
  onNext,
  showToast,
}: CalibrationStepProps) {
  const { t } = useTranslation();

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Activity size={16} className="text-[var(--cds-interactive)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('launcher.step2_heading')}</h2>
        </div>
        <span className="text-[var(--cds-text-helper)]">{t('launcher.step_counter', 'Step %s of 5', [2])}</span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.profile_strategy', 'Profile Strategy')}
          </label>
          <select 
            value={profile} 
            onChange={e => setProfile(e.target.value)}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
          >
            <option value="target_rps">{t('launcher.target_rps', 'Target RPS (Rate Limiting)')}</option>
            <option value="flat_vu">{t('launcher.flat_vu', 'Flat Virtual Users (Constant)')}</option>
            <option value="ramping">{t('launcher.ramping', 'Dynamic Ramping Stages')}</option>
          </select>
        </div>

        {profile !== 'ramping' && (
          <>
            {profile === 'target_rps' && (
              <div>
                <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
                  {t('launcher.target_rps_label', 'Target RPS')}
                </label>
                <input 
                  type="number" 
                  value={targetRPS} 
                  onChange={e => setTargetRPS(e.target.value)}
                  min={1} 
                  className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
                />
              </div>
            )}
            <div>
              <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
                {t('launcher.max_vus', 'Max Concurrent VUs')}
              </label>
              <input 
                type="number" 
                value={vus} 
                onChange={e => setVus(e.target.value)}
                min={1} 
                className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
              />
            </div>
            <div>
              <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
                {t('launcher.duration_sec', 'Test Duration (Seconds)')}
              </label>
              <input 
                type="number" 
                value={durationSec} 
                onChange={e => setDurationSec(e.target.value)}
                min={2} 
                className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
              />
            </div>
          </>
        )}
      </div>

      <CulturalPresetPicker onApplyPreset={onApplyCulturalPreset} showToast={showToast} />

      {/* Dynamic Ramping Stages Builder */}
      {profile === 'ramping' && (
        <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs text-[var(--cds-interactive)] font-semibold uppercase">
              {t('launcher.ramping_timeline', 'Multi-Stage Ramping Timeline')}
            </span>
            <button 
              type="button" 
              onClick={onAddStage}
              className="px-2.5 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-border-subtle)] text-white border border-[var(--cds-border-subtle)] text-xs"
            >
              {t('launcher.add_stage', '+ Add Stage')}
            </button>
          </div>
          <div className="space-y-2">
            {rampingStages.map((st, sIdx) => (
              <div key={sIdx} className="flex items-center space-x-3 bg-[var(--cds-background)] p-2 border border-[var(--cds-border-subtle)]">
                <span className="text-[var(--cds-text-helper)] w-20">{t('launcher.stage', 'Stage')} {sIdx + 1}</span>
                <div className="flex items-center space-x-1.5">
                  <span className="text-[10px] text-[var(--cds-text-helper)]">{t('launcher.duration', 'Duration')}:</span>
                  <input 
                    type="number" 
                    value={st.durationSec} 
                    onChange={e => onUpdateStage(sIdx, 'durationSec', e.target.value)}
                    min={1} 
                    className="w-16 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-1 text-white text-xs"
                  />
                  <span className="text-[var(--cds-text-helper)]">s</span>
                </div>
                <div className="flex items-center space-x-1.5">
                  <span className="text-[10px] text-[var(--cds-text-helper)]">{t('launcher.target_vus', 'Target VUs')}:</span>
                  <input 
                    type="number" 
                    value={st.targetVus} 
                    onChange={e => onUpdateStage(sIdx, 'targetVus', e.target.value)}
                    min={0} 
                    className="w-16 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-1 text-white text-xs"
                  />
                </div>
                <button 
                  type="button" 
                  onClick={() => onRemoveStage(sIdx)}
                  className="text-[var(--cds-support-error)] hover:underline text-[11px] ml-auto"
                >
                  {t('launcher.remove', 'Remove')}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

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
          <span>{t('launcher.next_step', 'Next Step')}: {t('launcher.step_3', 'Circuit Breaker')}</span>
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}
