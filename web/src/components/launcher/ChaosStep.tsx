import React from 'react';
import { Flash, ChevronLeft, ChevronRight } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';

export interface ChaosStepProps {
  chaosEnabled: boolean;
  setChaosEnabled: (val: boolean) => void;
  chaosDelay: number | string;
  setChaosDelay: (val: any) => void;
  chaosLatency: number | string;
  setChaosLatency: (val: any) => void;
  chaosJitter: number | string;
  setChaosJitter: (val: any) => void;
  chaosLoss: number | string;
  setChaosLoss: (val: any) => void;
  chaosTargetDomains: string;
  setChaosTargetDomains: (val: string) => void;
  dependencyPresets: any[];
  carrierPresets: any[];
  onApplyDependencyPreset: (id: string) => void;
  onApplyCarrierPreset: (id: string) => void;
  onPrev: () => void;
  onNext: () => void;
}

export default function ChaosStep({
  chaosEnabled,
  setChaosEnabled,
  chaosDelay,
  setChaosDelay,
  chaosLatency,
  setChaosLatency,
  chaosJitter,
  setChaosJitter,
  chaosLoss,
  setChaosLoss,
  chaosTargetDomains,
  setChaosTargetDomains,
  dependencyPresets,
  carrierPresets,
  onApplyDependencyPreset,
  onApplyCarrierPreset,
  onPrev,
  onNext,
}: ChaosStepProps) {
  const { t } = useTranslation();

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Flash size={16} className="text-[var(--cds-support-error)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('launcher.step4_heading')}</h2>
        </div>
        <label className="flex items-center space-x-2 cursor-pointer">
          <input 
            type="checkbox" 
            checked={chaosEnabled} 
            onChange={e => setChaosEnabled(e.target.checked)}
            className="accent-[var(--cds-support-error)]"
          />
          <span className="text-xs text-white font-medium">{t('launcher.enable_chaos', 'Enable Chaos Injection')}</span>
        </label>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.schedule_delay', 'Schedule Delay (sec)')}
          </label>
          <input 
            type="number" 
            value={chaosDelay} 
            onChange={e => setChaosDelay(e.target.value)}
            min={0}
            disabled={!chaosEnabled}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs disabled:opacity-30"
          />
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.injected_latency', 'Injected Latency (ms)')}
          </label>
          <input 
            type="number" 
            value={chaosLatency} 
            onChange={e => setChaosLatency(e.target.value)}
            min={0}
            disabled={!chaosEnabled}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs disabled:opacity-30"
          />
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.jitter_variance', 'Jitter Variance (ms)')}
          </label>
          <input 
            type="number" 
            value={chaosJitter} 
            onChange={e => setChaosJitter(e.target.value)}
            min={0}
            disabled={!chaosEnabled}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs disabled:opacity-30"
          />
        </div>
        <div>
          <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
            {t('launcher.packet_loss', 'Packet Loss (%)')}
          </label>
          <input 
            type="number" 
            value={chaosLoss} 
            onChange={e => setChaosLoss(e.target.value)}
            min={0}
            max={100}
            disabled={!chaosEnabled}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs disabled:opacity-30"
          />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-[var(--cds-border-subtle)]">
        <div>
          <label className="cds-label">{t('launcher.sim_dependency', 'Simulate Dependency Down')}</label>
          <select
            disabled={!chaosEnabled}
            onChange={e => e.target.value && onApplyDependencyPreset(e.target.value)}
            className="cds-select h-9 text-[11px] disabled:opacity-30"
            defaultValue=""
          >
            <option value="">— Select (Midtrans, Xendit, etc) —</option>
            {dependencyPresets.map((d: any) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </div>
        <div>
          <label className="cds-label">{t('launcher.sim_carrier', 'Simulate Network Operator')}</label>
          <select
            disabled={!chaosEnabled}
            onChange={e => e.target.value && onApplyCarrierPreset(e.target.value)}
            className="cds-select h-9 text-[11px] disabled:opacity-30"
            defaultValue=""
          >
            <option value="">— Select (Telkomsel/Indosat/XL) —</option>
            {carrierPresets.map((c: any) => <option key={c.id} value={c.id}>{c.operator} {c.generation}</option>)}
          </select>
        </div>
        <div className="md:col-span-2">
          <label className="cds-label">{t('launcher.target_domains', 'Target Domains')}</label>
          <input
            type="text"
            value={chaosTargetDomains}
            onChange={e => setChaosTargetDomains(e.target.value)}
            disabled={!chaosEnabled}
            placeholder="api.midtrans.com, api.xendit.co"
            className="cds-input h-9 text-[11px] disabled:opacity-30"
          />
          <span className="cds-helper-text">{t('launcher.target_domains_hint', 'Active with http_proxy driver. Empty = affects all traffic.')}</span>
        </div>
      </div>

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
          <span>{t('launcher.next_step', 'Next Step')}: {t('launcher.step_5', 'Spec & Launch')}</span>
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  );
}
