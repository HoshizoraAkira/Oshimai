import React from 'react';
import { useTranslation } from '../../context/I18nContext';
import { FlowNode, ToastType } from '../../types/models';

export interface CodeViewProps {
  rawYaml: string;
  setRawYaml: (val: string) => void;
  onSyncCodeToNodes: (nodes: FlowNode[], meta?: { scenarioId?: string; baseUrl?: string }) => void;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function CodeView({
  rawYaml,
  setRawYaml,
  onSyncCodeToNodes,
  showToast,
}: CodeViewProps) {
  const { t } = useTranslation();

  const handleSync = () => {
    try {
      const parsed = JSON.parse(rawYaml);
      if (!parsed.steps) {
        showToast(t('builder.toast_sync_missing_steps', 'JSON must include a "steps" object.'), 'error');
        return;
      }
      const newNodes: FlowNode[] = Object.entries(parsed.steps).map(([id, s]: [string, any], idx) => {
        // A step's shape tells us its node type — this mirrors buildScenarioObject's own
        // encoding in reverse. Guessing wrong here used to silently rewrite every decision/delay
        // step into a plain HTTP GET "/" on sync, destroying its condition/think_time.
        const hasRequest = !!s.request;
        const hasCondition = s.condition !== undefined;
        const type: FlowNode['type'] = hasRequest ? 'http' : hasCondition ? 'decision' : s.think_time !== undefined ? 'delay' : 'http';
        const thinkTimeMs = typeof s.think_time === 'string' ? parseInt(s.think_time, 10) : undefined;

        return {
          id: s.id || id,
          name: s.name || id,
          type,
          method: hasRequest ? (s.request?.method || 'GET') : undefined,
          path: hasRequest ? (s.request?.path || '/') : undefined,
          x: 80 + (idx % 4) * 440,
          y: 140 + Math.floor(idx / 4) * 240,
          isInitial: id === parsed.initial_step_id,
          headers: s.request?.headers || {},
          body: s.request?.body || '',
          condition: s.condition,
          delayMs: type === 'delay' ? (thinkTimeMs || 1000) : undefined,
          assertions: s.assertions || [],
          extractors: s.extractors || [],
          transitions: s.transitions || [{ target_step_id: 'END', probability: 1.0 }],
        };
      });
      onSyncCodeToNodes(newNodes, { scenarioId: parsed.id, baseUrl: parsed.base_url });
      showToast(t('builder.toast_sync_ok', 'Visual Canvas synchronized with code edits!'), 'success');
    } catch (e: any) {
      showToast(t('builder.toast_sync_err', 'Invalid JSON syntax: %s', [e.message]), 'error');
    }
  };

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-3">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2">
        <div>
          <span className="text-xs text-[var(--cds-support-success)] font-semibold uppercase">
            {t('builder.fsm_title', 'Finite-State Machine Specification')}
          </span>
          <p className="text-[11px] text-[var(--cds-text-helper)]">
            {t('builder.fsm_desc', 'Code is automatically generated from your visual drag-and-drop canvas. Manual editing is purely optional.')}
          </p>
        </div>
        <button
          onClick={handleSync}
          className="px-3 py-1 bg-[var(--cds-layer-02)] hover:bg-[var(--cds-layer-03)] text-xs text-[var(--cds-text-primary)] border border-[var(--cds-layer-03)]"
        >
          {t('builder.sync_code_btn', 'Sync Code to Visual Canvas')}
        </button>
      </div>

      <textarea
        rows={18}
        value={rawYaml}
        onChange={e => setRawYaml(e.target.value)}
        className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-3 text-xs text-[var(--cds-support-success)] font-mono leading-relaxed"
      />
    </div>
  );
}
