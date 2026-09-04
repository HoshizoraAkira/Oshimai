import React from 'react';
import {
  Add, Branch, Time, Notebook, Undo, Redo, Copy, Security,
  ZoomIn, ZoomOut, Reset, View, Upload, Code, ArrowRight, ArrowLeft, Save
} from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';

export interface FlowToolbarProps {
  scenarioId: string;
  setScenarioId: (id: string) => void;
  baseUrl: string;
  setBaseUrl: (url: string) => void;
  viewMode: 'visual' | 'code' | 'importer';
  setViewMode: (mode: 'visual' | 'code' | 'importer') => void;
  onSendToLauncher: () => void;
  sendToLauncherDisabled?: boolean;
  onBackToList: () => void;
  onSave: () => void;
  isEditingExisting: boolean;
  onAddNode: (type: 'http' | 'delay' | 'decision' | 'note', method?: string) => void;
  onUndo: () => void;
  onRedo: () => void;
  canUndo: boolean;
  canRedo: boolean;
  selectedNodeId: string | null;
  onDuplicateNode: (id: string) => void;
  onValidateFlow: () => void;
  zoom: number;
  setZoom: React.Dispatch<React.SetStateAction<number>>;
  onAutoLayout: () => void;
}

export default function FlowToolbar({
  scenarioId,
  setScenarioId,
  baseUrl,
  setBaseUrl,
  viewMode,
  setViewMode,
  onSendToLauncher,
  sendToLauncherDisabled,
  onBackToList,
  onSave,
  isEditingExisting,
  onAddNode,
  onUndo,
  onRedo,
  canUndo,
  canRedo,
  selectedNodeId,
  onDuplicateNode,
  onValidateFlow,
  zoom,
  setZoom,
  onAutoLayout,
}: FlowToolbarProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-2">
      {/* Top Action Bar */}
      <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center space-x-3">
          <button
            type="button"
            onClick={onBackToList}
            className="cds-btn-secondary cds-btn-sm normal-case shrink-0"
            title={t('builder.back_to_list', 'Back to Scenario List')}
          >
            <ArrowLeft size={14} />
            <span>{t('builder.back_to_list', 'Back to Scenario List')}</span>
          </button>
          <div className="flex items-center space-x-2">
            <span className="text-[var(--cds-text-helper)]">{t('builder.scenario_label', 'SCENARIO:')}</span>
            <input 
              type="text" 
              value={scenarioId} 
              onChange={e => setScenarioId(e.target.value)}
              className="bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs w-48"
            />
          </div>
          <div className="flex items-center space-x-2">
            <span className="text-[var(--cds-text-helper)]">{t('builder.target_base_url_label', 'TARGET BASE URL:')}</span>
            <input 
              type="text" 
              value={baseUrl} 
              onChange={e => setBaseUrl(e.target.value)}
              placeholder={t('builder.placeholder_target_base_url', 'https://api.internal')}
              className="bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs w-64"
            />
          </div>
        </div>

        {/* View Switcher & Action */}
        <div className="flex items-center space-x-2">
          <div className="inline-flex border border-[var(--cds-border-subtle)] bg-[var(--cds-background)]">
            <button 
              onClick={() => setViewMode('visual')}
              className={`px-3 py-1.5 flex items-center gap-1.5 transition-colors ${viewMode === 'visual' ? 'bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] font-semibold' : 'text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'}`}
            >
              <View size={14} className="text-[var(--cds-interactive)]" />
              <span>{t('builder.visual_canvas', 'Visual Canvas')}</span>
            </button>
            <button 
              onClick={() => setViewMode('importer')}
              className={`px-3 py-1.5 flex items-center gap-1.5 transition-colors border-l border-[var(--cds-border-subtle)] ${viewMode === 'importer' ? 'bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] font-semibold' : 'text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'}`}
            >
              <Upload size={14} className="text-[var(--cds-support-novel)]" />
              <span>{t('builder.import_spec', 'Import OpenAPI/OTel')}</span>
            </button>
            <button
              onClick={() => setViewMode('code')}
              className={`px-3 py-1.5 flex items-center gap-1.5 transition-colors border-l border-[var(--cds-border-subtle)] ${viewMode === 'code' ? 'bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] font-semibold' : 'text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]'}`}
            >
              <Code size={14} className="text-[var(--cds-support-success)]" />
              <span>{t('builder.raw_code', 'Raw Code')}</span>
            </button>
          </div>

          <button
            onClick={onSave}
            className="cds-btn-secondary cds-btn-sm normal-case"
          >
            <Save size={14} />
            <span>{isEditingExisting ? t('builder.save_changes', 'Save Changes') : t('builder.save_as_new', 'Save as New Scenario')}</span>
          </button>

          <button
            onClick={onSendToLauncher}
            disabled={sendToLauncherDisabled}
            title={sendToLauncherDisabled ? t('builder.title_send_to_launcher_disabled', 'Add at least one step with a designated START before executing.') : undefined}
            className="bg-[var(--cds-interactive)] hover:bg-[var(--cds-interactive-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-[var(--cds-text-primary)] px-3.5 py-1.5 flex items-center gap-1.5 font-semibold transition-colors"
          >
            <span>{t('builder.execute_in_launcher', 'Execute in Launcher')}</span>
            <ArrowRight size={14} />
          </button>
        </div>
      </div>

      {/* Canvas Controls Bar (visible only in visual canvas mode) */}
      {viewMode === 'visual' && (
        <div className="flex flex-wrap items-center justify-between gap-3 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-4 py-2 shrink-0">
          {/* Palette: Add Nodes */}
          <div className="flex items-center space-x-1.5">
            <span className="text-[var(--cds-text-helper)] uppercase text-[10px] mr-1">{t('builder.tools_label', 'Tools:')}</span>
            <button 
              onClick={() => onAddNode('http', 'GET')}
              className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-link)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_add_get', 'Add HTTP GET step')}
            >
              <Add size={14} /> GET
            </button>
            <button 
              onClick={() => onAddNode('http', 'POST')}
              className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-support-success-text)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_add_post', 'Add HTTP POST step')}
            >
              <Add size={14} /> POST
            </button>
            <button 
              onClick={() => onAddNode('decision')}
              className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-support-warning)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_add_decision', 'Add conditional decision branching')}
            >
              <Branch size={14} /> {t('builder.node_decision', 'Decision')}
            </button>
            <button 
              onClick={() => onAddNode('delay')}
              className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-support-novel-text)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_add_delay', 'Add think time delay')}
            >
              <Time size={14} /> {t('builder.node_think_time', 'Think Time')}
            </button>
            <button 
              onClick={() => onAddNode('note')}
              className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-text-secondary)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_add_note', 'Add architecture sticky note')}
            >
              <Notebook size={14} /> {t('builder.node_note', 'Note')}
            </button>
          </div>

          {/* Canvas Controls: Undo/Redo, Zoom, Clone, Validate */}
          <div className="flex items-center space-x-2">
            <button 
              onClick={onUndo} 
              disabled={!canUndo}
              className="p-1.5 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] disabled:opacity-30 border border-[var(--cds-border-subtle)] text-[var(--cds-text-primary)]"
              title={t('builder.title_undo', 'Undo (Ctrl+Z)')}
            >
              <Undo size={14} />
            </button>
            <button 
              onClick={onRedo} 
              disabled={!canRedo}
              className="p-1.5 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] disabled:opacity-30 border border-[var(--cds-border-subtle)] text-[var(--cds-text-primary)]"
              title={t('builder.title_redo', 'Redo (Ctrl+Y)')}
            >
              <Redo size={14} />
            </button>

            <span className="text-[var(--cds-border-subtle)]">|</span>

            {selectedNodeId && (
              <button 
                onClick={() => onDuplicateNode(selectedNodeId)}
                className="px-2 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] border border-[var(--cds-border-subtle)] text-[var(--cds-text-primary)] flex items-center gap-1"
                title={t('builder.title_duplicate', 'Duplicate selected step')}
              >
                <Copy size={14} /> {t('builder.clone', 'Clone')}
              </button>
            )}

            <button 
              onClick={onValidateFlow}
              className="px-2.5 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] border border-[var(--cds-border-subtle)] text-[var(--cds-support-success-text)] flex items-center gap-1 font-semibold"
              title={t('builder.title_validate', 'Validate flow integrity and transition coherence')}
            >
              <Security size={14} /> {t('builder.validate', 'Validate')}
            </button>

            <span className="text-[var(--cds-border-subtle)]">|</span>

            <button 
              onClick={() => setZoom(z => Math.max(0.6, z - 0.1))}
              className="p-1.5 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] border border-[var(--cds-border-subtle)] text-[var(--cds-text-primary)]"
              title={t('builder.title_zoom_out', 'Zoom Out')}
            >
              <ZoomOut size={14} />
            </button>
            <button 
              onClick={() => setZoom(1.0)}
              className="px-1.5 py-1 text-[10px] text-[var(--cds-text-helper)] hover:text-[var(--cds-text-primary)]"
              title={t('builder.title_zoom_reset', 'Reset Zoom')}
            >
              {(zoom * 100).toFixed(0)}%
            </button>
            <button 
              onClick={() => setZoom(z => Math.min(1.5, z + 0.1))}
              className="p-1.5 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] border border-[var(--cds-border-subtle)] text-[var(--cds-text-primary)]"
              title={t('builder.title_zoom_in', 'Zoom In')}
            >
              <ZoomIn size={14} />
            </button>

            <button 
              onClick={onAutoLayout}
              className="px-2.5 py-1 bg-[var(--cds-layer-01)] hover:bg-[var(--cds-layer-02)] text-[var(--cds-text-primary)] border border-[var(--cds-border-subtle)] flex items-center gap-1"
              title={t('builder.title_auto_layout', 'Auto-organize layout')}
            >
              <Reset size={14} className="text-[var(--cds-interactive)]" /> {t('builder.auto_layout', 'Auto-Layout')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
