import React from 'react';
import {
  Code, DocumentImport, Search, MagicWand, PlayFilled, ChevronLeft, FolderOpen, Layers, Checkmark
} from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { SavedScenario } from '../../types/models';

export interface SpecStepProps {
  targetBaseUrl: string;
  profile: string;
  targetRPS: number | string;
  vus: number | string;
  cbErrorRate: number | string;
  cbP99Ms: number | string;
  chaosEnabled: boolean;
  chaosLatency: number | string;
  chaosLoss: number | string;
  customScenario: string;
  setCustomScenario: (val: string) => void;
  nlDescription: string;
  setNlDescription: (val: string) => void;
  importBusy: boolean;
  isLaunching: boolean;
  onImportFile: (endpoint: string, field: string, file: File) => void;
  onAutoDiscover: () => void;
  onGenerateFromDescription: () => void;
  savedScenarios: SavedScenario[];
  onLoadSavedScenario: (s: SavedScenario) => void;
  onPrev: () => void;
}

export default function SpecStep({
  targetBaseUrl,
  profile,
  targetRPS,
  vus,
  cbErrorRate,
  cbP99Ms,
  chaosEnabled,
  chaosLatency,
  chaosLoss,
  customScenario,
  setCustomScenario,
  nlDescription,
  setNlDescription,
  importBusy,
  isLaunching,
  onImportFile,
  onAutoDiscover,
  onGenerateFromDescription,
  savedScenarios,
  onLoadSavedScenario,
  onPrev,
}: SpecStepProps) {
  const { t } = useTranslation();

  const importOptions = [
    { label: 'HAR', accept: '.har,.json', endpoint: '/api/v1/scenarios/import/har', field: 'har' },
    { label: 'Postman', accept: '.json', endpoint: '/api/v1/scenarios/import/postman', field: 'collection' },
    { label: 'Insomnia', accept: '.json', endpoint: '/api/v1/scenarios/import/insomnia', field: 'export' },
    { label: 'JMeter (.jmx)', accept: '.jmx,.xml', endpoint: '/api/v1/scenarios/import/jmeter', field: 'jmx' },
    { label: 'k6 Script', accept: '.js', endpoint: '/api/v1/scenarios/import/k6', field: 'script' },
  ];

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div className="flex items-center space-x-2">
          <Code size={16} className="text-[var(--cds-interactive)]" />
          <h2 className="text-xs font-semibold uppercase text-white">{t('launcher.step5_heading')}</h2>
        </div>
        <span className="text-[var(--cds-text-helper)]">{t('launcher.step_counter', 'Step %s of 5', [5])}</span>
      </div>

      {/* Pre-flight Configuration Summary Card */}
      <div className="bg-[var(--cds-surface-sunken)] border border-[var(--cds-interactive)] p-3 space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-[11px] text-[var(--cds-interactive)] font-bold uppercase tracking-wider inline-flex items-center gap-1.5">
            <Checkmark size={14} />
            <span>{t('launcher.summary_title', 'Pre-flight Configuration Summary')}</span>
          </span>
          <span className="text-[10px] text-[var(--cds-support-success)] font-semibold uppercase">{t('launcher.ready_to_launch')}</span>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-[11px] pt-1 border-t border-[#2d2d2d]">
          <div>
            <span className="text-[var(--cds-text-helper)] block text-[10px] uppercase">{t('launcher.summary_target')}:</span>
            <span className="text-white font-mono truncate block">{targetBaseUrl || 'https://httpbin.org'}</span>
          </div>
          <div>
            <span className="text-[var(--cds-text-helper)] block text-[10px] uppercase">{t('launcher.summary_workload')}:</span>
            <span className="text-white font-semibold block">
              {profile === 'target_rps' ? `${targetRPS} RPS` : profile === 'flat_vu' ? `${vus} VUs` : 'Ramping'}
            </span>
          </div>
          <div>
            <span className="text-[var(--cds-text-helper)] block text-[10px] uppercase">{t('launcher.summary_safety')}:</span>
            <span className="text-white font-semibold block">{cbErrorRate}% err | {cbP99Ms}ms P99</span>
          </div>
          <div>
            <span className="text-[var(--cds-text-helper)] block text-[10px] uppercase">{t('launcher.summary_chaos')}:</span>
            <span className={chaosEnabled ? 'text-[var(--cds-support-error)] font-semibold block' : 'text-[var(--cds-text-helper)] block'}>
              {chaosEnabled ? `+${chaosLatency}ms (${chaosLoss}% loss)` : 'Off'}
            </span>
          </div>
        </div>
      </div>

      {/* My Scenarios — templates saved from the Scenario Builder's library */}
      <div className="bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] p-3 space-y-2">
        <div className="flex items-center gap-1.5 text-[10px] text-[var(--cds-text-helper)] uppercase">
          <Layers size={13} className="text-[var(--cds-link)]" />
          <span>{t('launcher.my_scenarios_label', 'My Scenarios:')}</span>
        </div>
        {savedScenarios.length === 0 ? (
          <p className="text-[11px] text-[var(--cds-text-helper)] italic">
            {t('launcher.no_saved_scenarios_hint', 'No saved scenarios yet — build one in Scenario Builder → Scenario Library and save it as a template.')}
          </p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {savedScenarios.map(s => (
              <button
                key={s.id}
                type="button"
                onClick={() => onLoadSavedScenario(s)}
                className="cds-btn-secondary cds-btn-sm normal-case"
                title={t('launcher.load_saved_scenario', 'Load "%s" into the spec below', [s.name])}
              >
                <FolderOpen size={14} />
                <span>{s.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Zero-spec Import Buttons */}
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[10px] text-[var(--cds-text-helper)] uppercase mr-1">
          {t('launcher.zero_spec_import', 'Zero-Spec Import:')}
        </span>
        {importOptions.map(imp => (
          <label key={imp.label} className="cds-btn-secondary cds-btn-sm normal-case cursor-pointer">
            <DocumentImport size={14} />
            <span>{imp.label}</span>
            <input
              type="file"
              accept={imp.accept}
              className="hidden"
              disabled={importBusy}
              onChange={e => {
                const file = e.target.files?.[0];
                if (file) onImportFile(imp.endpoint, imp.field, file);
              }}
            />
          </label>
        ))}
        <button
          type="button"
          onClick={onAutoDiscover}
          disabled={importBusy}
          className="cds-btn-tertiary cds-btn-sm normal-case"
        >
          <Search size={14} />
          <span>{importBusy ? t('common.loading', 'Processing...') : t('launcher.autodiscover', '1-Click Auto-Discover')}</span>
        </button>
      </div>

      <div className="flex items-center gap-2">
        <input
          type="text"
          value={nlDescription}
          onChange={e => setNlDescription(e.target.value)}
          placeholder={t('launcher.nl_placeholder', 'Or describe your app: "online store with login, browse, checkout"')}
          className="cds-input flex-1 h-9 text-[11px]"
        />
        <button
          type="button"
          onClick={onGenerateFromDescription}
          disabled={importBusy}
          className="cds-btn-secondary cds-btn-sm normal-case shrink-0"
        >
          <MagicWand size={14} />
          <span>{t('launcher.generate_nl', 'Generate from Description')}</span>
        </button>
      </div>

      <div>
        <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">
          {t('launcher.scenario_spec_label', 'Scenario Specification')} <span className="text-[var(--cds-support-error)]">*</span>
        </label>
        <textarea
          rows={8}
          value={customScenario}
          onChange={e => setCustomScenario(e.target.value)}
          placeholder={t('launcher.yaml_placeholder', 'Paste Scenario YAML or JSON...')}
          className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-3 text-xs text-[var(--cds-support-success)] font-mono leading-relaxed resize-y"
        />
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
          type="submit"
          disabled={isLaunching || !customScenario.trim()}
          title={!customScenario.trim() ? t('launcher.toast_scenario_spec_required', 'Scenario specification is required.') : undefined}
          className="px-8 py-3 bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-white font-bold flex items-center justify-center gap-2 text-sm transition-colors shadow-lg"
        >
          <PlayFilled size={16} />
          <span>{isLaunching ? t('launcher.initiating', 'Initiating...') : t('launcher.start_btn', 'Start Test Run')}</span>
        </button>
      </div>
    </div>
  );
}
