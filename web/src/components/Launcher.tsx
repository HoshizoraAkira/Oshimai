import React, { useState, useEffect } from 'react';
import { ChevronLeft, Checkmark } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { presetsService } from '../services/presetsService';
import { scenarioService } from '../services/scenarioService';
import { scenarioLibrary } from '../services/scenarioLibrary';
import { buildScenarioObject } from '../utils/scenarioBuilder';
import { secToNs, msToNs, nsToSec } from '../utils/units';
import { SavedScenario, ToastType, TargetVerifyStatus } from '../types/models';

import TargetStep from './launcher/TargetStep';
import CalibrationStep from './launcher/CalibrationStep';
import SafeguardsStep from './launcher/SafeguardsStep';
import ChaosStep from './launcher/ChaosStep';
import SpecStep from './launcher/SpecStep';
import ApprovalModal from './launcher/ApprovalModal';
import BreakingPointPanel from './BreakingPointPanel';

export interface LauncherProps {
  scenarioYaml?: string;
  baseUrl?: string;
  onRunStarted: (runId: string) => void;
  showToast: (msg: string, type?: ToastType) => void;
  activeSubTab?: string;
}

export default function Launcher({
  scenarioYaml = '',
  baseUrl = '',
  onRunStarted,
  showToast,
  activeSubTab,
}: LauncherProps) {
  const { t } = useTranslation();

  // Process / Stepper state (1 to 5, or 6 for breaking point)
  const [currentStep, setCurrentStep] = useState(1);

  // Sync activeSubTab to step if provided
  useEffect(() => {
    if (!activeSubTab) return;
    const tabMap: Record<string, number> = {
      target: 1,
      calibration: 2,
      safeguards: 3,
      chaos: 4,
      spec: 5,
      breaking: 6,
    };
    if (tabMap[activeSubTab]) {
      setCurrentStep(tabMap[activeSubTab]);
    }
  }, [activeSubTab]);

  // Target & Environment
  const [targetBaseUrl, setTargetBaseUrl] = useState(baseUrl || '');
  const [environment, setEnvironment] = useState('');
  const [verifyStatus, setVerifyStatus] = useState<TargetVerifyStatus | null>(null);
  const needsVerification = !!verifyStatus && !verifyStatus.is_private && !verifyStatus.is_verified;
  const [awaitingApproval, setAwaitingApproval] = useState<any>(null);
  const [approverName, setApproverName] = useState('');
  const [approving, setApproving] = useState(false);

  // Business impact translation (optional)
  const [avgTxnValue, setAvgTxnValue] = useState<number | string>('');
  const [txnPerMinute, setTxnPerMinute] = useState<number | string>('');
  const [benchmarkCategory, setBenchmarkCategory] = useState('');

  // Zero-spec onboarding imports
  const [importBusy, setImportBusy] = useState(false);
  const [nlDescription, setNlDescription] = useState('');

  useEffect(() => {
    if (baseUrl) setTargetBaseUrl(baseUrl);
  }, [baseUrl]);

  const [profile, setProfile] = useState('target_rps');
  const [targetRPS, setTargetRPS] = useState<number | string>(50);
  const [vus, setVus] = useState<number | string>(20);
  const [durationSec, setDurationSec] = useState<number | string>(15);
  
  // Dynamic Ramping Stages
  const [rampingStages, setRampingStages] = useState([
    { durationSec: 5, targetVus: 10 },
    { durationSec: 10, targetVus: 25 },
    { durationSec: 5, targetVus: 0 },
  ]);

  // Circuit Breaker Thresholds
  const [cbErrorRate, setCbErrorRate] = useState<number | string>(10);
  const [cbP99Ms, setCbP99Ms] = useState<number | string>(500);

  // Chaos Injection State
  const [chaosEnabled, setChaosEnabled] = useState(false);
  const [chaosDelay, setChaosDelay] = useState<number | string>(3);
  const [chaosLatency, setChaosLatency] = useState<number | string>(300);
  const [chaosJitter, setChaosJitter] = useState<number | string>(50);
  const [chaosLoss, setChaosLoss] = useState<number | string>(5);
  const [chaosTargetDomains, setChaosTargetDomains] = useState('');

  // Presets
  const [dependencyPresets, setDependencyPresets] = useState<any[]>([]);
  const [carrierPresets, setCarrierPresets] = useState<any[]>([]);

  // Scenario definition
  const [customScenario, setCustomScenario] = useState('');
  const [isLaunching, setIsLaunching] = useState(false);

  useEffect(() => {
    if (scenarioYaml) {
      setCustomScenario(scenarioYaml);
    }
  }, [scenarioYaml]);

  // User's saved scenario templates (from Scenario Builder's library) — lets an
  // operator try more than one scenario without re-pasting or rebuilding it each time.
  const [savedScenarios, setSavedScenarios] = useState<SavedScenario[]>(() => scenarioLibrary.list());
  useEffect(() => {
    if (currentStep === 5) setSavedScenarios(scenarioLibrary.list());
  }, [currentStep]);

  const handleLoadSavedScenario = (s: SavedScenario) => {
    const scenarioObj = buildScenarioObject(s.nodes, s.scenarioId, s.baseUrl);
    setCustomScenario(JSON.stringify(scenarioObj, null, 2));
    if (s.baseUrl) setTargetBaseUrl(s.baseUrl);
    showToast(t('launcher.toast_loaded_template', 'Loaded scenario template "%s".', [s.name]), 'success');
  };

  // Fetch presets
  useEffect(() => {
    presetsService.getDependencyPresets()
      .then(data => setDependencyPresets(data || []))
      .catch(() => showToast(t('launcher.toast_load_optional_presets_err', 'Failed to load some optional presets.'), 'warning'));
    presetsService.getCarrierPresets()
      .then(data => setCarrierPresets(data || []))
      .catch(() => showToast(t('launcher.toast_load_optional_presets_err', 'Failed to load some optional presets.'), 'warning'));
  }, []);

  const handleAddStage = () => {
    setRampingStages(prev => [...prev, { durationSec: 10, targetVus: 20 }]);
  };

  const handleRemoveStage = (idx: number) => {
    setRampingStages(prev => prev.filter((_, i) => i !== idx));
  };

  const handleUpdateStage = (idx: number, field: string, value: any) => {
    setRampingStages(prev => prev.map((st, i) => {
      if (i !== idx) return st;
      return { ...st, [field]: parseInt(value) || 0 };
    }));
  };

  // Fetches the server-computed ramping curve for this preset (scaled from the operator's current
  // baseline VU count) instead of reimplementing each preset's shape client-side — that curve
  // depends on CulturalPreset.BuildRampingStages, including any custom Shape a user-created preset
  // may define, which only the server knows how to expand correctly.
  const applyCulturalPreset = async (preset: any) => {
    const baseline = parseInt(String(vus)) || 10;
    try {
      const { stages } = await presetsService.getCulturalStages(preset.id, baseline);
      setVus(baseline);
      setProfile('ramping');
      setRampingStages(
        (stages || []).map(s => ({
          durationSec: Math.max(1, Math.round(nsToSec(s.duration))),
          targetVus: s.target_vus,
        }))
      );
      showToast(t('launcher.toast_preset_applied', 'Cultural preset "%s" applied — %s ramping stage(s) generated.', [preset.name, (stages || []).length]), 'success');
    } catch (err: any) {
      showToast(t('launcher.toast_preset_apply_err', 'Failed to apply preset: %s', [err.message]), 'error');
    }
  };

  const applyDependencyPreset = (presetId: string) => {
    const p = dependencyPresets.find(x => x.id === presetId);
    if (!p) return;
    setChaosEnabled(true);
    setChaosDelay(p.recommended_delay_seconds || 3);
    setChaosLatency(p.latency_ms || 0);
    setChaosLoss(p.packet_loss_percent || 0);
    setChaosTargetDomains(p.target_domain || '');
    showToast(t('launcher.toast_dep_preset_applied', 'Applied preset: %s', [p.name]), 'info');
  };

  const applyCarrierPreset = (presetId: string) => {
    const c = carrierPresets.find(x => x.id === presetId);
    if (!c) return;
    setChaosEnabled(true);
    setChaosLatency(c.latency_ms || 0);
    setChaosJitter(c.jitter_ms || 0);
    setChaosLoss(c.packet_loss_percent || 0);
    showToast(t('launcher.toast_carrier_preset_applied', 'Applied network preset: %s %s', [c.operator, c.generation]), 'info');
  };

  const handleImportFile = async (endpoint: string, field: string, file: File) => {
    if (!file) return;
    setImportBusy(true);
    try {
      const data = await scenarioService.importFile(file, field);
      setCustomScenario(typeof data === 'string' ? data : JSON.stringify(data, null, 2));
      showToast(t('launcher.toast_imported', 'Imported scenario from %s!', [file.name]), 'success');
    } catch (err: any) {
      showToast(t('launcher.toast_import_err', 'Import failed: %s', [err.message]), 'error');
    } finally {
      setImportBusy(false);
    }
  };

  const handleAutoDiscover = async () => {
    if (!targetBaseUrl.trim()) {
      showToast(t('toasts.target_url_required', 'Target Base URL is required for auto-discovery.'), 'error');
      setCurrentStep(1);
      return;
    }
    setImportBusy(true);
    try {
      const data = await scenarioService.autoDiscover(targetBaseUrl);
      setCustomScenario(typeof data === 'string' ? data : JSON.stringify(data, null, 2));
      showToast(t('toasts.autodiscover_ok', 'Auto-discovery complete! Generated scenario with detected endpoints.'), 'success');
    } catch (err: any) {
      showToast(t('toasts.autodiscover_err', 'Auto-discovery failed: %s', [err.message]), 'error');
    } finally {
      setImportBusy(false);
    }
  };

  const handleGenerateFromDescription = async () => {
    if (!nlDescription.trim()) {
      showToast(t('toasts.describe_app', 'Describe your application flow first.'), 'warning');
      return;
    }
    setImportBusy(true);
    try {
      const data = await scenarioService.generateFromPrompt(nlDescription, targetBaseUrl || undefined);
      setCustomScenario(data.scenario_yaml || JSON.stringify(data.scenario, null, 2));
      showToast(t('toasts.scenario_generated', 'Scenario generated from description!'), 'success');
    } catch (err: any) {
      showToast(t('toasts.generate_err', 'Generation failed: %s', [err.message]), 'error');
    } finally {
      setImportBusy(false);
    }
  };

  const handleApprove = async () => {
    if (!awaitingApproval || !approverName.trim()) {
      showToast(t('launcher.toast_approver_name_required', 'Approver name is required.'), 'error');
      return;
    }
    setApproving(true);
    try {
      const targetId = awaitingApproval.run_id || awaitingApproval.id;
      const data = await runsService.approveRun(targetId, approverName);
      showToast(t('launcher.toast_approved', 'Run %s approved by %s.', [targetId, approverName]), 'success');
      setAwaitingApproval(null);
      onRunStarted(targetId);
    } catch (err: any) {
      showToast(t('launcher.toast_approval_err', 'Approval failed: %s', [err.message]), 'error');
    } finally {
      setApproving(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!customScenario.trim()) {
      showToast(t('launcher.toast_scenario_spec_required', 'Scenario specification is required.'), 'error');
      setCurrentStep(5);
      return;
    }
    if (needsVerification) {
      showToast(t('launcher.verification_required_hint', 'Verify target ownership before continuing.'), 'error');
      setCurrentStep(1);
      return;
    }

    setIsLaunching(true);

    const stagesPayload = profile === 'ramping'
      ? rampingStages.map(s => ({ duration: secToNs(Number(s.durationSec)), target_vus: Number(s.targetVus) }))
      : undefined;

    const payload: any = {
      target_base_url: targetBaseUrl,
      environment: environment,
      load_config: {
        profile: profile,
        target_rps: parseFloat(String(targetRPS)) || 20,
        vus: parseInt(String(vus)) || 10,
        duration: secToNs(parseInt(String(durationSec))),
        stages: stagesPayload,
        circuit_breaker: {
          enabled: true,
          evaluation_interval: 1e9,
          window_duration: 3e9,
          min_requests: 5,
          max_error_rate: parseFloat(String(cbErrorRate)) / 100,
          max_p99_latency: msToNs(parseInt(String(cbP99Ms))),
          consecutive_breaches: 1,
        }
      },
      chaos_plan: {
        enabled: chaosEnabled,
        schedule_delay: secToNs(parseInt(String(chaosDelay))),
        fault: {
          id: 'launcher_simulated_fault',
          type: 'composite',
          parameters: {
            latency_ms: parseInt(String(chaosLatency)) || 0,
            jitter_ms: parseInt(String(chaosJitter)) || 0,
            loss_percent: parseFloat(String(chaosLoss)) || 0,
            target_domains: chaosTargetDomains.trim()
              ? chaosTargetDomains.split(',').map(s => s.trim()).filter(Boolean)
              : [],
          }
        }
      },
      scenario_yaml: customScenario,
      business_impact: (avgTxnValue || txnPerMinute) ? {
        average_transaction_value: parseFloat(String(avgTxnValue)) || 0,
        estimated_transactions_per_minute: parseFloat(String(txnPerMinute)) || 0,
        currency: 'IDR',
      } : undefined,
      benchmark_category: benchmarkCategory || undefined,
    };

    try {
      const created: any = await runsService.createRun(payload);
      const runId = created.run_id || created.id;
      if (created.status === 'awaiting_approval') {
        setAwaitingApproval(created);
        showToast(t('launcher.toast_awaiting_2nd_approval', 'Run %s targets production — waiting for 2nd approval.', [runId]), 'warning');
        return;
      }
      showToast(t('launcher.toast_run_initiated', 'Test Run initiated: %s', [runId]), 'success');
      onRunStarted(runId);
    } catch (err: any) {
      showToast(t('launcher.toast_launch_err', 'Launch failed: %s', [err.message]), 'error');
    } finally {
      setIsLaunching(false);
    }
  };

  const STEPS = [
    { num: 1, label: t('launcher.step_1', 'Target & Scope') },
    { num: 2, label: t('launcher.step_2', 'Workload Profile') },
    { num: 3, label: t('launcher.step_3', 'Circuit Breaker') },
    { num: 4, label: t('launcher.step_4', 'Chaos Injection') },
    { num: 5, label: t('launcher.step_5', 'Spec & Launch') },
  ];

  return (
    <form onSubmit={handleSubmit} className="space-y-4 font-mono text-xs">
      {/* Continuity Stepper / Process Bar */}
      <div className="bg-[var(--cds-surface-header)] border border-[var(--cds-border-subtle)] p-3">
        <div className="flex items-center justify-between gap-2 overflow-x-auto pb-1">
          {STEPS.map((s) => {
            const isCurrent = currentStep === s.num;
            const isCompleted = currentStep > s.num;
            const isBlocked = s.num > 1 && needsVerification;

            return (
              <button
                key={s.num}
                type="button"
                onClick={() => { if (!isBlocked) setCurrentStep(s.num); }}
                disabled={isBlocked}
                title={isBlocked ? t('launcher.verification_required_hint', 'Verify target ownership before continuing.') : undefined}
                className={`flex items-center gap-2.5 px-3 py-2 text-left shrink-0 transition-all border ${
                  isBlocked
                    ? 'opacity-40 cursor-not-allowed bg-transparent border-transparent text-[var(--cds-border-strong)]'
                    : isCurrent
                    ? 'bg-[var(--cds-layer-01)] border-[var(--cds-interactive)] text-white shadow-sm'
                    : isCompleted
                    ? 'bg-[#181818] border-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)] hover:text-white'
                    : 'bg-transparent border-transparent text-[var(--cds-border-strong)] hover:text-[#a8a8a8]'
                }`}
              >
                <span className={`w-5 h-5 flex items-center justify-center text-[10px] font-bold rounded-full ${
                  isCurrent
                    ? 'bg-[var(--cds-interactive)] text-white'
                    : isCompleted
                    ? 'bg-[var(--cds-support-success)] text-black'
                    : 'bg-[#333] text-[var(--cds-text-helper)]'
                }`}>
                  {isCompleted ? <Checkmark size={12} /> : s.num}
                </span>
                <span className="text-xs font-medium uppercase tracking-wider">{s.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      <ApprovalModal
        awaitingApproval={awaitingApproval}
        approverName={approverName}
        setApproverName={setApproverName}
        approving={approving}
        onApprove={handleApprove}
      />

      {currentStep === 1 && (
        <TargetStep
          targetBaseUrl={targetBaseUrl}
          setTargetBaseUrl={setTargetBaseUrl}
          environment={environment}
          setEnvironment={setEnvironment}
          avgTxnValue={avgTxnValue}
          setAvgTxnValue={setAvgTxnValue}
          txnPerMinute={txnPerMinute}
          setTxnPerMinute={setTxnPerMinute}
          benchmarkCategory={benchmarkCategory}
          setBenchmarkCategory={setBenchmarkCategory}
          verifyStatus={verifyStatus}
          onVerifyStatusChange={setVerifyStatus}
          onNext={() => setCurrentStep(2)}
          showToast={showToast}
        />
      )}

      {currentStep === 2 && (
        <CalibrationStep
          profile={profile}
          setProfile={setProfile}
          targetRPS={targetRPS}
          setTargetRPS={setTargetRPS}
          vus={vus}
          setVus={setVus}
          durationSec={durationSec}
          setDurationSec={setDurationSec}
          rampingStages={rampingStages}
          onAddStage={handleAddStage}
          onRemoveStage={handleRemoveStage}
          onUpdateStage={handleUpdateStage}
          onApplyCulturalPreset={applyCulturalPreset}
          onPrev={() => setCurrentStep(1)}
          onNext={() => setCurrentStep(3)}
          showToast={showToast}
        />
      )}

      {currentStep === 3 && (
        <SafeguardsStep
          cbErrorRate={cbErrorRate}
          setCbErrorRate={setCbErrorRate}
          cbP99Ms={cbP99Ms}
          setCbP99Ms={setCbP99Ms}
          onPrev={() => setCurrentStep(2)}
          onNext={() => setCurrentStep(4)}
        />
      )}

      {currentStep === 4 && (
        <ChaosStep
          chaosEnabled={chaosEnabled}
          setChaosEnabled={setChaosEnabled}
          chaosDelay={chaosDelay}
          setChaosDelay={setChaosDelay}
          chaosLatency={chaosLatency}
          setChaosLatency={setChaosLatency}
          chaosJitter={chaosJitter}
          setChaosJitter={setChaosJitter}
          chaosLoss={chaosLoss}
          setChaosLoss={setChaosLoss}
          chaosTargetDomains={chaosTargetDomains}
          setChaosTargetDomains={setChaosTargetDomains}
          dependencyPresets={dependencyPresets}
          carrierPresets={carrierPresets}
          onApplyDependencyPreset={applyDependencyPreset}
          onApplyCarrierPreset={applyCarrierPreset}
          onPrev={() => setCurrentStep(3)}
          onNext={() => setCurrentStep(5)}
        />
      )}

      {currentStep === 5 && (
        <SpecStep
          targetBaseUrl={targetBaseUrl}
          profile={profile}
          targetRPS={targetRPS}
          vus={vus}
          cbErrorRate={cbErrorRate}
          cbP99Ms={cbP99Ms}
          chaosEnabled={chaosEnabled}
          chaosLatency={chaosLatency}
          chaosLoss={chaosLoss}
          customScenario={customScenario}
          setCustomScenario={setCustomScenario}
          nlDescription={nlDescription}
          setNlDescription={setNlDescription}
          importBusy={importBusy}
          isLaunching={isLaunching}
          onImportFile={handleImportFile}
          onAutoDiscover={handleAutoDiscover}
          onGenerateFromDescription={handleGenerateFromDescription}
          savedScenarios={savedScenarios}
          onLoadSavedScenario={handleLoadSavedScenario}
          onPrev={() => setCurrentStep(4)}
        />
      )}

      {currentStep === 6 && (
        <div className="space-y-4">
          <BreakingPointPanel
            scenarioText={customScenario}
            targetBaseUrl={targetBaseUrl}
            vus={vus}
            showToast={showToast}
          />
          <div className="flex justify-start">
            <button
              type="button"
              onClick={() => setCurrentStep(1)}
              className="px-3 py-2 bg-[var(--cds-layer-01)] hover:bg-[#333] border border-[var(--cds-border-subtle)] text-white flex items-center gap-1.5"
            >
              <ChevronLeft size={14} />
              <span>{t('launcher.back_step_1', 'Back to Step 1')}</span>
            </button>
          </div>
        </div>
      )}
    </form>
  );
}
