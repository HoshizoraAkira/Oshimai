import React, { useState } from 'react';
import { useTranslation } from '../context/I18nContext';
import { ToastType } from '../types/models';
import AgentsPanel from './ops/AgentsPanel';
import K8sPanel from './ops/K8sPanel';
import GameDayPanel from './ops/GameDayPanel';
import CloudVerifyPanel from './ops/CloudVerifyPanel';
import BenchmarkPanel from './ops/BenchmarkPanel';
import ReplayPanel from './ops/ReplayPanel';
import PresetsPanel from './ops/PresetsPanel';

export interface OpsConsoleProps {
  showToast: (msg: string, type?: ToastType) => void;
  onSendToLauncher: (yaml: string, baseUrl?: string) => void;
  activeSubTab?: string;
  onSelectSubTab?: (subTab: string) => void;
}

export default function OpsConsole({
  showToast,
  onSendToLauncher,
  activeSubTab,
  onSelectSubTab,
}: OpsConsoleProps) {
  const { t } = useTranslation();
  const subtabs = [
    { id: 'agents', label: t('nav.agents', 'Agents & Multi-Region') },
    { id: 'k8s', label: t('nav.k8s', 'Kubernetes Chaos') },
    { id: 'gameday', label: t('nav.gameday', 'GameDay Schedules') },
    { id: 'cloudverify', label: t('nav.cloudverify', 'Cloud Verification') },
    { id: 'benchmark', label: t('nav.benchmark', 'Benchmark Compare') },
    { id: 'replay', label: t('nav.replay', 'Trace Replay') },
    { id: 'presets', label: t('nav.presets', 'Cultural Presets') },
  ];
  const validIds = subtabs.map(s => s.id);

  const [internalSubTab, setInternalSubTab] = useState('agents');
  const subTab = (activeSubTab && validIds.includes(activeSubTab)) ? activeSubTab : internalSubTab;
  const setSubTab = onSelectSubTab || setInternalSubTab;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap border-b border-[var(--cds-border-subtle)]">
        {subtabs.map(item => (
          <button
            key={item.id}
            type="button"
            onClick={() => setSubTab(item.id)}
            className={`px-4 h-10 flex items-center text-xs font-mono font-semibold uppercase tracking-wide border-b-2 transition-colors ${
              subTab === item.id
                ? 'border-[var(--cds-interactive)] text-[var(--cds-link)]'
                : 'border-transparent text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {subTab === 'agents' && <AgentsPanel showToast={showToast} />}
      {subTab === 'k8s' && <K8sPanel showToast={showToast} />}
      {subTab === 'gameday' && <GameDayPanel showToast={showToast} />}
      {subTab === 'cloudverify' && <CloudVerifyPanel showToast={showToast} />}
      {subTab === 'benchmark' && <BenchmarkPanel showToast={showToast} />}
      {subTab === 'replay' && <ReplayPanel showToast={showToast} onSendToLauncher={onSendToLauncher} />}
      {subTab === 'presets' && <PresetsPanel showToast={showToast} />}
    </div>
  );
}
