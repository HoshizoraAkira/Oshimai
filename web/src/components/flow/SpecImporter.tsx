import React, { useState } from 'react';
import { ChartNetwork, WatsonMachineLearning } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { scenarioService } from '../../services/scenarioService';
import { FlowNode, ToastType } from '../../types/models';
import { DependencyGraphData } from '../../types/api';

export interface SpecImporterProps {
  baseUrl: string;
  onImportSuccess: (nodes: FlowNode[], scenarioId: string) => void;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function SpecImporter({
  baseUrl,
  onImportSuccess,
  showToast,
}: SpecImporterProps) {
  const { t } = useTranslation();
  const [openApiInput, setOpenApiInput] = useState('');
  const [otelInput, setOtelInput] = useState('');
  const [dependencyGraph, setDependencyGraph] = useState<DependencyGraphData | null>(null);
  const [isBuildingGraph, setIsBuildingGraph] = useState(false);
  const [isSynthesizing, setIsSynthesizing] = useState(false);

  const handleSynthesizeFromSpec = async () => {
    if (!openApiInput.trim()) {
      showToast(t('builder.toast_need_openapi', 'Please provide an OpenAPI v3 specification.'), 'error');
      return;
    }
    setIsSynthesizing(true);
    try {
      const sc = await scenarioService.generateScenario({
        openapi_spec: openApiInput,
        otel_traces: otelInput,
        config: {
          scenario_id: 'synthesized_' + Date.now(),
          base_url: baseUrl,
        }
      });

      const newNodes: FlowNode[] = Object.entries(sc.steps || {}).map(([id, step], idx) => ({
        id: step.id || id,
        name: step.name || id,
        type: 'http',
        method: step.request?.method || 'GET',
        path: step.request?.path || '/',
        x: 80 + (idx % 4) * 440,
        y: 140 + Math.floor(idx / 4) * 240,
        isInitial: id === sc.initial_step_id,
        headers: step.request?.headers || {},
        authBearer: '',
        body: step.request?.body || '',
        assertions: [{ type: 'status_in_range', min_code: 200, max_code: 200 }],
        extractors: step.extractors || [],
        transitions: step.transitions || [{ target_step_id: 'END', probability: 1.0 }]
      }));

      onImportSuccess(newNodes, sc.id || 'synthesized_journey');
      showToast(t('builder.toast_mined', 'Mined %s steps directly into Visual Canvas!', [newNodes.length]), 'success');
    } catch (err: any) {
      showToast(t('builder.toast_synthesis_err', 'Synthesis error: %s', [err.message]), 'error');
    } finally {
      setIsSynthesizing(false);
    }
  };

  const handleBuildDependencyGraph = async () => {
    if (!otelInput.trim()) {
      showToast(t('builder.toast_need_otel', 'Paste OTel traces above first — the dependency graph is mined from real traffic.'), 'error');
      return;
    }
    setIsBuildingGraph(true);
    try {
      const data = await scenarioService.getDependencyGraph(otelInput);
      setDependencyGraph(data);
      showToast(t('builder.toast_graph_found', 'Found %s endpoint(s) — ranked by criticality.', [data.nodes?.length || 0]), 'success');
    } catch (err: any) {
      showToast(t('builder.toast_graph_err', 'Dependency graph error: %s', [err.message]), 'error');
    } finally {
      setIsBuildingGraph(false);
    }
  };

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 space-y-4">
      <div className="border-b border-[var(--cds-border-subtle)] pb-2">
        <span className="text-xs text-[var(--cds-support-novel)] font-semibold uppercase">
          {t('builder.synthesize_title', 'Synthesize Visual Scenario from OpenAPI & OTel')}
        </span>
        <p className="text-[11px] text-[var(--cds-text-helper)]">
          {t('builder.synthesize_desc', 'Upload an OpenAPI schema and trace recordings to automatically extract endpoints and generate the visual flow graph.')}
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="space-y-2">
          <label className="text-xs text-[var(--cds-text-primary)] font-semibold">
            {t('builder.openapi_spec_label', '1. OpenAPI Specification (YAML / JSON)')}
          </label>
          <textarea 
            rows={10}
            value={openApiInput}
            onChange={e => setOpenApiInput(e.target.value)}
            placeholder={t('builder.openapi_placeholder', 'Paste OpenAPI v3 YAML/JSON or upload file...')}
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-xs text-[var(--cds-support-success)] font-mono"
          />
        </div>
        <div className="space-y-2">
          <label className="text-xs text-[var(--cds-text-primary)] font-semibold">
            {t('builder.otel_traces_label', '2. OpenTelemetry Traces (JSON Array, Optional)')}
          </label>
          <textarea 
            rows={10}
            value={otelInput}
            onChange={e => setOtelInput(e.target.value)}
            placeholder='[{"trace_id": "t1", "route": "/api/v1/auth/login", "method": "POST"}]'
            className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-xs text-[var(--cds-support-success)] font-mono"
          />
        </div>
      </div>

      <div className="flex items-center justify-end space-x-3 pt-2">
        <button
          disabled={isBuildingGraph}
          onClick={handleBuildDependencyGraph}
          className="px-4 py-2 bg-[var(--cds-layer-02)] hover:bg-[var(--cds-layer-03)] text-[var(--cds-text-primary)] text-xs font-semibold flex items-center gap-2"
          title={t('builder.title_dependency_graph', 'PageRank-style endpoint dependency graph')}
        >
          <ChartNetwork size={16} />
          <span>{isBuildingGraph ? t('common.loading', 'Analyzing...') : t('builder.view_dependency_graph', 'View Dependency Graph')}</span>
        </button>
        <button
          disabled={isSynthesizing}
          onClick={handleSynthesizeFromSpec}
          className="px-4 py-2 bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] text-[var(--cds-text-primary)] text-xs font-semibold flex items-center gap-2"
        >
          <WatsonMachineLearning size={16} />
          <span>{isSynthesizing ? t('common.loading', 'Synthesizing...') : t('builder.synthesize_btn', 'Synthesize to Visual Flow Canvas')}</span>
        </button>
      </div>

      {dependencyGraph && dependencyGraph.nodes && dependencyGraph.nodes.length > 0 && (
        <div className="border-t border-[var(--cds-border-subtle)] pt-3 space-y-2">
          <span className="text-xs text-[var(--cds-support-novel)] font-semibold uppercase">{t('builder.critical_endpoints_title')}</span>
          <div className="space-y-1.5">
            {dependencyGraph.nodes.slice(0, 8).map(node => (
              <div key={node.id} className="flex items-center gap-2 text-[11px] font-mono">
                <span className="w-56 truncate text-[var(--cds-text-primary)]" title={node.id}>{node.id}</span>
                <div className="flex-1 h-2 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)]">
                  <div className="h-full bg-[var(--cds-support-novel)]" style={{ width: `${Math.max(node.criticality * 100, 2)}%` }} />
                </div>
                <span className="w-12 text-right text-[var(--cds-text-helper)]">{(node.criticality * 100).toFixed(0)}%</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
