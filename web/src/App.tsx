import React, { useState, useEffect } from 'react';
import { Menu } from '@carbon/icons-react';
import VisualFlowBuilder from './components/VisualFlowBuilder';
import LiveCockpit from './components/LiveCockpit';
import Launcher from './components/Launcher';
import HistoryView from './components/HistoryView';
import OpsConsole from './components/OpsConsole';
import ExecutionSummaryDrawer from './components/ExecutionSummaryDrawer';
import RemediationModal from './components/RemediationModal';
import WelcomeModal from './components/WelcomeModal';
import ComplianceCertificateModal from './components/ComplianceCertificateModal';
import PublicStatusPage from './components/PublicStatusPage';
import PageHeader from './components/PageHeader';
import SidebarNav from './components/SidebarNav';
import AdminDock from './components/AdminDock';
import ToastContainer from './components/common/ToastContainer';
import { I18nProvider, useTranslation } from './context/I18nContext';
import { useToast } from './hooks/useToast';
import { runsService } from './services/runsService';
import { Run } from './types/models';

const TERMINAL_RUN_STATUSES = ['completed', 'aborted', 'failed'];

function AppShell() {
  const { t } = useTranslation();
  const { toasts, showToast, dismissToast } = useToast();

  // Navigation state — restored from sessionStorage on refresh
  const [activeTab, setActiveTab] = useState<string>(() => sessionStorage.getItem('oshimai_activeTab') || 'launcher');
  const [activeSubTab, setActiveSubTab] = useState<string>(() => sessionStorage.getItem('oshimai_activeSubTab') || 'target');

  // Admin Dock & Logs collapse state — starts collapsed so the log terminal
  // doesn't cover the bottom of the workspace on first load; the operator
  // expands it explicitly when they want to watch backend logs.
  const [isDockCollapsed, setIsDockCollapsed] = useState(true);

  // Off-canvas sidebar state — the nav is a fixed drawer below the `lg`
  // breakpoint so it never eats into the workspace width on phones/tablets.
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  // Sidebar rail collapse (desktop only) — persisted so the operator's
  // preferred layout survives a refresh.
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState<boolean>(
    () => sessionStorage.getItem('oshimai_sidebarCollapsed') === '1'
  );
  useEffect(() => {
    sessionStorage.setItem('oshimai_sidebarCollapsed', isSidebarCollapsed ? '1' : '0');
  }, [isSidebarCollapsed]);

  const [currentRunId, setCurrentRunId] = useState<string | null>(() => sessionStorage.getItem('oshimai_currentRunId'));
  // Whether currentRunId refers to a run that's actually still in flight — a runId
  // alone (e.g. restored from sessionStorage, or reopened from History) doesn't mean
  // the run hasn't already finished, so the Admin Dock kill-switch must gate on this
  // instead of just runId truthiness.
  const [isRunActive, setIsRunActive] = useState(false);
  const [launcherScenario, setLauncherScenario] = useState('');
  const [targetBaseUrl, setTargetBaseUrl] = useState('https://httpbin.org');

  // Modals & Drawers
  const [drawerRunId, setDrawerRunId] = useState<string | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [modalRunId, setModalRunId] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Guidance & Compliance Modals
  const [isWelcomeOpen, setIsWelcomeOpen] = useState(false);
  const [isComplianceOpen, setIsComplianceOpen] = useState(false);
  const [complianceRunData, setComplianceRunData] = useState<Run | null>(null);

  // Persist navigation state to sessionStorage so refresh restores position
  useEffect(() => { sessionStorage.setItem('oshimai_activeTab', activeTab); }, [activeTab]);
  useEffect(() => { sessionStorage.setItem('oshimai_activeSubTab', activeSubTab); }, [activeSubTab]);
  useEffect(() => {
    if (currentRunId) {
      sessionStorage.setItem('oshimai_currentRunId', currentRunId);
    } else {
      sessionStorage.removeItem('oshimai_currentRunId');
    }
  }, [currentRunId]);

  // Confirm the actual status of currentRunId whenever it changes — covers reopening
  // an already-finished run from History, and a page reload restoring a runId whose
  // run finished while the tab was closed.
  useEffect(() => {
    if (!currentRunId) {
      setIsRunActive(false);
      return;
    }
    let cancelled = false;
    runsService.getRun(currentRunId)
      .then((run) => {
        if (!cancelled) setIsRunActive(!TERMINAL_RUN_STATUSES.includes(run.status));
      })
      .catch(() => {
        // Transient lookup failure — leave the current assumption in place.
      });
    return () => { cancelled = true; };
  }, [currentRunId]);

  // First-time visit onboarding
  useEffect(() => {
    const onboarded = localStorage.getItem('oshimai_onboarded');
    if (!onboarded) {
      setIsWelcomeOpen(true);
    }
  }, []);

  const handleSendToLauncher = (yaml: string, baseUrl?: string) => {
    setLauncherScenario(yaml);
    if (baseUrl) setTargetBaseUrl(baseUrl);
    setActiveTab('launcher');
    setActiveSubTab('spec');
    showToast(t('toasts.scenario_transferred', 'Scenario transferred to Run Launcher.'), 'info');
  };

  const handleRunStarted = (runId: string) => {
    setCurrentRunId(runId);
    setIsRunActive(true);
    setActiveTab('cockpit');
    setActiveSubTab('telemetry');
  };

  // Reopen a past run's telemetry cockpit from History & Trends — LiveCockpit
  // detects the run has already finished and renders its final report instead
  // of waiting on a live stream.
  const handleViewCockpit = (runId: string) => {
    setCurrentRunId(runId);
    setActiveTab('cockpit');
    setActiveSubTab('telemetry');
  };

  const handleAbortRun = async (runId: string) => {
    if (!runId) return;
    if (!confirm(`Emergency abort execution for ${runId}?`)) return;

    try {
      await runsService.abortRun(runId);
      setIsRunActive(false);
      showToast(`Run ${runId} aborted by operator kill-switch.`, 'warning');
    } catch (e: any) {
      showToast(`Abort error: ${e.message}`, 'error');
    }
  };

  const openDrawer = (runId: string) => {
    setDrawerRunId(runId);
    setIsDrawerOpen(true);
  };

  const openModal = (runId: string) => {
    setModalRunId(runId);
    setIsModalOpen(true);
  };

  const tabHeaders: Record<string, { eyebrow: string; title: string; description: string }> = {
    launcher: {
      eyebrow: `Oshimai // ${t('app.launcher_eyebrow', 'Controlled Execution')}`,
      title: t('app.launcher_title', 'Launch Load & Chaos Test'),
      description: t('app.launcher_desc', 'Verify target ownership, calibrate workload concurrency, schedule chaos injection, and trigger execution with autonomous circuit breaker.'),
    },
    builder: {
      eyebrow: `Oshimai // ${t('app.builder_eyebrow', 'Visual Authoring')}`,
      title: t('app.builder_title', 'Design User Journey Scenario'),
      description: t('app.builder_desc', 'Compose drag-and-drop requests or synthesize automatically from OpenAPI, OTel trace, HAR, or Postman collection.'),
    },
    cockpit: {
      eyebrow: `Oshimai // ${t('app.cockpit_eyebrow', 'Real-Time Telemetry')}`,
      title: t('app.cockpit_title', 'Live Execution Telemetry'),
      description: t('app.cockpit_desc', 'RPS, error rate, and P50/P90/P99 latency updated every 500ms via Server-Sent Events.'),
    },
    history: {
      eyebrow: `Oshimai // ${t('app.history_eyebrow', 'Audit Trail')}`,
      title: t('app.history_title', 'Execution History & Trends'),
      description: t('app.history_desc', 'Compare test health against baseline runs, share public status links, or grab the Battle-Tested badge.'),
    },
    ops: {
      eyebrow: `Oshimai // ${t('app.ops_eyebrow', 'Enterprise Operations')}`,
      title: t('app.ops_title', 'Multi-Region, K8s Chaos & Compliance'),
      description: t('app.ops_desc', 'Manage multi-region agent fleet, inject Kubernetes chaos, schedule GameDay exercises, and compare industry benchmarks.'),
    },
  };

  const currentHeader = tabHeaders[activeTab] || tabHeaders.launcher;

  return (
    <div className="min-h-screen flex bg-[var(--cds-background)] text-[var(--cds-text-primary)] font-sans antialiased selection:bg-[var(--cds-interactive)] selection:text-white">

      {/* Toast Notification Container */}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />

      {/* Sidebar Navigation */}
      <SidebarNav
        activeMenu={activeTab}
        onSelectMenu={(menu: string) => {
          setActiveTab(menu);
          if (menu === 'ops') {
            setActiveSubTab('agents');
          }
        }}
        onOpenGuide={() => setIsWelcomeOpen(true)}
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
        isCollapsed={isSidebarCollapsed}
        onToggleCollapse={() => setIsSidebarCollapsed(v => !v)}
      />

      {/* Main Workspace Area (Panel style, bottom-dock aware) */}
      <div className={`flex-1 flex flex-col min-w-0 transition-all ${isDockCollapsed ? 'pb-12' : 'pb-60'}`}>
        {/* Mobile-only top bar: hamburger opens the off-canvas nav drawer */}
        <div className="lg:hidden h-12 shrink-0 border-b border-[var(--cds-border-subtle)] bg-[#161616] px-4 flex items-center gap-3">
          <button
            type="button"
            onClick={() => setIsSidebarOpen(true)}
            className="p-1 text-[#c6c6c6] hover:text-white"
            title="Open navigation"
          >
            <Menu size={20} />
          </button>
          <span className="font-bold font-mono tracking-wider text-sm text-white">OSHIMAI</span>
        </div>

        <main className="flex-1 w-full max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-5 flex flex-col">
          {currentHeader && (
            <PageHeader
              eyebrow={currentHeader.eyebrow}
              title={currentHeader.title}
              description={currentHeader.description}
            />
          )}

          {activeTab === 'launcher' && (
            <Launcher
              scenarioYaml={launcherScenario}
              baseUrl={targetBaseUrl}
              onRunStarted={handleRunStarted}
              showToast={showToast}
              activeSubTab={activeSubTab}
            />
          )}

          {activeTab === 'builder' && (
            <VisualFlowBuilder
              onSendToLauncher={handleSendToLauncher}
              showToast={showToast}
              activeSubTab={activeSubTab}
              onSelectSubTab={(s) => setActiveSubTab(s)}
            />
          )}

          {activeTab === 'cockpit' && (
            <LiveCockpit
              currentRunId={currentRunId}
              onAbortRun={handleAbortRun}
              onRunActiveChange={setIsRunActive}
              onOpenSummaryDrawer={openDrawer}
              onOpenDoctorModal={openModal}
              onOpenComplianceCertificate={(data) => {
                setComplianceRunData(data);
                setIsComplianceOpen(true);
              }}
              showToast={showToast}
              activeSubTab={activeSubTab}
            />
          )}

          {activeTab === 'history' && (
            <HistoryView
              onInspectRun={openDrawer}
              onOpenAdvisory={openModal}
              onViewCockpit={handleViewCockpit}
              showToast={showToast}
              activeSubTab={activeSubTab}
            />
          )}

          {activeTab === 'ops' && (
            <OpsConsole
              showToast={showToast}
              onSendToLauncher={handleSendToLauncher}
              activeSubTab={activeSubTab}
              onSelectSubTab={(s) => setActiveSubTab(s)}
            />
          )}
        </main>
      </div>

      {/* Go Backend Logs & Control Dock (Fixed to Bottom) */}
      <AdminDock
        currentRunId={currentRunId}
        isRunActive={isRunActive}
        onAbortRun={handleAbortRun}
        showToast={showToast}
        targetBaseUrl={targetBaseUrl}
        vus={20}
        isCollapsed={isDockCollapsed}
        onToggleCollapse={setIsDockCollapsed}
        sidebarCollapsed={isSidebarCollapsed}
      />

      {/* Slide-Over Drawer: Execution Summary */}
      <ExecutionSummaryDrawer
        runId={drawerRunId}
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        onOpenDoctorModal={(id) => {
          setIsDrawerOpen(false);
          openModal(id);
        }}
      />

      {/* Modal: Remediation Advisory */}
      <RemediationModal
        runId={modalRunId}
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        showToast={showToast}
      />

      {/* Modal: Guidance OOBE Walkthrough */}
      <WelcomeModal
        isOpen={isWelcomeOpen}
        onClose={() => setIsWelcomeOpen(false)}
      />

      {/* Modal: Compliance & ISO Audit Certificate */}
      <ComplianceCertificateModal
        run={complianceRunData}
        diagnostics={complianceRunData?.diagnostics}
        isOpen={isComplianceOpen}
        onClose={() => setIsComplianceOpen(false)}
      />

    </div>
  );
}

export default function App() {
  const publicToken = new URLSearchParams(window.location.search).get('public');
  if (publicToken) {
    return (
      <I18nProvider>
        <PublicStatusPage token={publicToken} />
      </I18nProvider>
    );
  }

  return (
    <I18nProvider>
      <AppShell />
    </I18nProvider>
  );
}
