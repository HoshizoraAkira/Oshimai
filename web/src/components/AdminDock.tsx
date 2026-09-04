import React, { useState } from 'react';
import {
  Terminal, Reset, ChevronUp, ChevronDown, TrashCan
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { useSystemLogs } from '../hooks/useSystemLogs';

export interface AdminDockProps {
  currentRunId?: string | null;
  isRunActive?: boolean;
  onAbortRun?: (runId: string) => void;
  targetBaseUrl?: string;
  vus?: number | string;
  isCollapsed?: boolean;
  onToggleCollapse?: (collapsed: boolean) => void;
  sidebarCollapsed?: boolean;
}

export default function AdminDock({
  currentRunId,
  isRunActive,
  onAbortRun,
  targetBaseUrl,
  vus,
  isCollapsed: controlledCollapsed,
  onToggleCollapse,
  sidebarCollapsed,
}: AdminDockProps) {
  const { t } = useTranslation();
  const [internalCollapsed, setInternalCollapsed] = useState(false);
  const isCollapsed = controlledCollapsed !== undefined ? controlledCollapsed : internalCollapsed;

  const toggleCollapse = () => {
    const next = !isCollapsed;
    if (onToggleCollapse) onToggleCollapse(next);
    setInternalCollapsed(next);
  };

  const {
    logs: filteredLogs,
    autoScroll,
    setAutoScroll,
    filterLevel,
    setFilterLevel,
    isConnected,
    clearLogs: handleClearLogs,
    logContainerRef,
  } = useSystemLogs(isCollapsed);

  const latestLog = filteredLogs.length > 0 ? filteredLogs[filteredLogs.length - 1] : null;

  return (
    <aside
      className={`fixed bottom-0 left-0 ${sidebarCollapsed ? 'lg:left-16' : 'lg:left-64'} right-0 z-30 bg-[var(--cds-background)] border-t border-[var(--cds-border-subtle)] font-mono text-xs select-none shadow-2xl transition-all duration-200 ${
        isCollapsed ? 'h-9' : 'h-56 sm:h-60'
      } flex flex-col`}
      aria-label="Golang Backend Logs & Admin Dock"
    >
      {/* Header bar */}
      <div className="h-9 px-3 bg-[var(--cds-surface-deep)] border-b border-[var(--cds-layer-01)] flex items-center justify-between font-mono text-[11px] shrink-0">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex items-center gap-1.5 text-white font-semibold">
            <Terminal size={14} className="text-[var(--cds-interactive)] shrink-0" />
            <span className="uppercase tracking-wider truncate">
              {t('admin.title', 'Backend Logs')}
            </span>
          </div>

          {/* Connection Status Pill */}
          <div className="hidden sm:flex items-center gap-1 px-1.5 py-0.5 bg-[var(--cds-surface-header)] border border-[#333] text-[10px]">
            <span
              className={`w-1.5 h-1.5 rounded-full ${
                isConnected ? 'bg-[var(--cds-support-success)] pulse-dot-green' : 'bg-[var(--cds-support-warning)]'
              }`}
            ></span>
            <span className={isConnected ? 'text-[var(--cds-support-success-text)]' : 'text-[var(--cds-support-warning)]'}>
              {isConnected ? 'LIVE STREAM' : t('admin.system_standby')}
            </span>
          </div>

          {/* Target & VUs Info */}
          <div className="hidden md:flex items-center gap-3 text-[10px] text-[var(--cds-text-helper)]">
            <span>
              {t('admin.target_label')}: <span className="text-white font-mono">{targetBaseUrl || 'Default'}</span>
            </span>
            <span>
              {t('admin.vus_label')}: <span className="text-[var(--cds-interactive)] font-semibold">{vus || 20}</span>
            </span>
            {currentRunId && isRunActive && (
              <span className="text-[var(--cds-support-warning)] font-semibold flex items-center gap-1 animate-pulse">
                <span>{t('admin.active_run_label')}:</span>
                <span className="underline">{currentRunId.slice(0, 8)}...</span>
              </span>
            )}
          </div>

          {/* Ticker when collapsed */}
          {isCollapsed && latestLog && (
            <div
              onClick={toggleCollapse}
              className="truncate text-[10px] text-[#a8a8a8] cursor-pointer hover:text-white max-w-[280px] sm:max-w-md ml-2 hidden sm:block"
              title={t('admin.title_expand_logs', 'Click to expand logs')}
            >
              <span className="text-[var(--cds-border-strong)]">[{latestLog.timestamp}]</span> {latestLog.message}
            </div>
          )}
        </div>

        {/* Action Controls */}
        <div className="flex items-center gap-2 shrink-0">
          {/* Filter Level Pills */}
          {!isCollapsed && (
            <div className="flex items-center bg-[var(--cds-surface-header)] border border-[#333] p-0.5 text-[10px]">
              {['ALL', 'INFO', 'WARN', 'ERROR'].map((lvl) => (
                <button
                  key={lvl}
                  type="button"
                  onClick={() => setFilterLevel(lvl)}
                  className={`px-1.5 py-0.5 font-semibold ${
                    filterLevel === lvl
                      ? 'bg-[var(--cds-interactive)] text-white'
                      : 'text-[var(--cds-text-helper)] hover:text-white'
                  }`}
                >
                  {lvl}
                </button>
              ))}
            </div>
          )}

          {/* Auto-scroll toggle */}
          {!isCollapsed && (
            <button
              type="button"
              onClick={() => setAutoScroll(!autoScroll)}
              className={`px-2 py-0.5 text-[10px] border hidden sm:block transition-colors ${
                autoScroll
                  ? 'bg-[var(--cds-interactive)]/20 text-[var(--cds-link)] border-[var(--cds-interactive)]'
                  : 'bg-[var(--cds-surface-header)] text-[var(--cds-text-helper)] border-[#333] hover:text-white'
              }`}
              title={t('common.title_toggle_autoscroll', 'Toggle Auto-Scroll')}
            >
              Auto-Scroll: {autoScroll ? 'ON' : 'OFF'}
            </button>
          )}

          {/* Clear Logs Button */}
          {!isCollapsed && (
            <button
              type="button"
              onClick={handleClearLogs}
              className="p-1 text-[var(--cds-text-helper)] hover:text-white hover:bg-[var(--cds-layer-01)] transition-colors"
              title={t('admin.title_clear_logs', 'Clear Console Logs')}
            >
              <TrashCan size={14} />
            </button>
          )}

          {/* Emergency Kill-Switch */}
          {currentRunId && isRunActive && (
            <button
              type="button"
              onClick={() => onAbortRun && onAbortRun(currentRunId)}
              className="px-2 py-0.5 bg-[var(--cds-support-error)] hover:bg-[var(--cds-support-error-hover)] text-white font-semibold text-[10px] flex items-center gap-1 shrink-0"
              title={t('admin.title_abort_run', 'Abort Active Run')}
            >
              <Reset size={12} />
              <span className="hidden sm:inline">{t('admin.abort_killswitch')}</span>
            </button>
          )}

          {/* Collapse / Expand Button */}
          <button
            type="button"
            onClick={toggleCollapse}
            className="p-1 text-[var(--cds-text-helper)] hover:text-white hover:bg-[var(--cds-layer-01)] transition-colors"
            title={isCollapsed ? 'Expand Terminal Logs' : 'Minimize Terminal Logs'}
          >
            {isCollapsed ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>
      </div>

      {/* Terminal Log Console Body */}
      {!isCollapsed && (
        <div
          ref={logContainerRef}
          className="flex-1 bg-[#0d0d0d] overflow-y-auto p-3 font-mono text-[11px] leading-relaxed space-y-0.5 select-text"
        >
          {filteredLogs.length === 0 ? (
            <div className="text-[var(--cds-border-strong)] italic p-4 text-center">
              No backend log events recorded yet. Logs emitted by the Go server will stream here live.
            </div>
          ) : (
            filteredLogs.map((entry, idx) => {
              const lvl = (entry.level || 'INFO').toUpperCase();
              const badgeStyle =
                lvl === 'ERROR'
                  ? 'text-[var(--cds-support-error-text)] bg-[var(--cds-support-error)]/20 border border-[var(--cds-support-error)]/40'
                  : lvl === 'WARN'
                  ? 'text-[var(--cds-support-warning)] bg-[var(--cds-support-warning)]/20 border border-[var(--cds-support-warning)]/40'
                  : 'text-[var(--cds-link)] bg-[var(--cds-interactive)]/15 border border-[var(--cds-interactive)]/30';

              return (
                <div key={idx} className="flex items-start gap-2 hover:bg-[var(--cds-background)] px-1 py-0.5 rounded">
                  <span className="text-[#666] shrink-0 select-none text-[10px] font-mono pt-0.5">
                    {entry.timestamp}
                  </span>
                  <span className={`px-1 py-0.5 text-[9px] font-bold uppercase shrink-0 ${badgeStyle}`}>
                    {lvl}
                  </span>
                  <span className="text-[#e0e0e0] font-mono break-all whitespace-pre-wrap">
                    {entry.message}
                  </span>
                </div>
              );
            })
          )}
        </div>
      )}
    </aside>
  );
}
