import React from 'react';
import { Settings, Password, Security } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { FlowNode } from '../../types/models';
import { getMethodBadge } from './FlowNodeCard';

export interface NodeInspectorProps {
  selectedNode: FlowNode | null;
  nodes: FlowNode[];
  onUpdateNode: (fields: Partial<FlowNode>) => void;
  onSetInitialNode: (nodeId: string, checked: boolean) => void;
}

export default function NodeInspector({
  selectedNode,
  nodes,
  onUpdateNode,
  onSetInitialNode,
}: NodeInspectorProps) {
  const { t } = useTranslation();

  return (
    <div className="w-full lg:w-[380px] bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-4 flex flex-col font-mono text-xs h-full shrink-0">
      <div className="flex items-center justify-between border-b border-[var(--cds-border-subtle)] pb-2.5 shrink-0">
        <div className="flex items-center space-x-2">
          <Settings size={16} className="text-[var(--cds-interactive)]" />
          <span className="font-semibold text-[var(--cds-text-primary)] uppercase tracking-wider">{t('builder.inspector_title', 'Step Inspector')}</span>
        </div>
        {selectedNode && (
          <span className={`text-[10px] px-2 py-0.5 border font-bold ${getMethodBadge(selectedNode.method || 'STEP')}`}>
            {selectedNode.type?.toUpperCase()}
          </span>
        )}
      </div>

      {selectedNode ? (
        <div className="space-y-3.5 overflow-y-auto flex-1 pr-1.5 mt-3">
          <div>
            <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">{t('builder.field_step_id', 'Step Identifier')}</label>
            <input 
              type="text" 
              value={selectedNode.id} 
              onChange={e => onUpdateNode({ id: e.target.value })}
              className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs"
            />
          </div>

          <div>
            <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">{t('builder.field_display_name', 'Display Name')}</label>
            <input 
              type="text" 
              value={selectedNode.name} 
              onChange={e => onUpdateNode({ name: e.target.value })}
              className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs"
            />
          </div>

          {/* HTTP-specific inputs */}
          {selectedNode.type === 'http' && (
            <>
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">{t('builder.field_method', 'Method')}</label>
                  <select 
                    value={selectedNode.method} 
                    onChange={e => onUpdateNode({ method: e.target.value })}
                    className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs"
                  >
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                    <option value="PUT">PUT</option>
                    <option value="DELETE">DELETE</option>
                  </select>
                </div>
                <div className="col-span-2">
                  <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">{t('builder.field_path', 'Path')}</label>
                  <input 
                    type="text" 
                    value={selectedNode.path} 
                    onChange={e => onUpdateNode({ path: e.target.value })}
                    className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs"
                  />
                </div>
              </div>

              {/* Auth Bearer Token Preset */}
              <div>
                <label className="text-[10px] text-[var(--cds-support-warning)] uppercase block mb-1 flex items-center gap-1">
                  <Password size={12} /> {t('builder.field_auth_bearer', 'Authorization Bearer Token')}
                </label>
                <input
                  type="text"
                  value={selectedNode.authBearer || ''}
                  onChange={e => onUpdateNode({ authBearer: e.target.value })}
                  placeholder={t('builder.placeholder_auth_bearer', 'Bearer token or {{auth_token}} var')}
                  className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs font-mono"
                />
              </div>

              {/* Step Assertions */}
              <div className="p-2 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-[10px] text-[var(--cds-support-success-text)] uppercase font-bold flex items-center gap-1">
                    <Security size={12} /> {t('builder.assertions_title', 'Response Assertions')}
                  </span>
                </div>
                <div className="flex items-center space-x-2">
                  <span className="text-[var(--cds-text-helper)]">{t('builder.expected_status', 'Expected Status:')}</span>
                  <input
                    type="text"
                    value={selectedNode.assertions?.[0]?.max_code || 200}
                    onChange={e => {
                      const code = parseInt(e.target.value) || 200;
                      onUpdateNode({ assertions: [{ type: 'status_in_range', min_code: code, max_code: code }] });
                    }}
                    className="w-16 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-1 py-0.5 text-[var(--cds-text-primary)]"
                  />
                </div>
              </div>

              {/* Request Payload */}
              {selectedNode.method !== 'GET' && (
                <div>
                  <label className="text-[10px] text-[var(--cds-text-helper)] uppercase block mb-1">{t('builder.field_request_body', 'Request Body (JSON)')}</label>
                  <textarea 
                    rows={4}
                    value={selectedNode.body || ''}
                    onChange={e => onUpdateNode({ body: e.target.value })}
                    placeholder='{"key": "value"}'
                    className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-xs text-[var(--cds-support-success)] font-mono resize-y"
                  />
                </div>
              )}
            </>
          )}

          {/* Delay Node Inputs */}
          {selectedNode.type === 'delay' && (
            <div>
              <label className="text-[10px] text-[var(--cds-support-novel-text)] uppercase block mb-1">{t('builder.field_think_time', 'Think Time Delay (ms)')}</label>
              <input 
                type="number" 
                value={selectedNode.delayMs || 1000} 
                onChange={e => onUpdateNode({ delayMs: parseInt(e.target.value) || 500 })}
                min={100}
                className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] px-2 py-1 text-[var(--cds-text-primary)] text-xs"
              />
            </div>
          )}

          {/* Note Node Input */}
          {selectedNode.type === 'note' && (
            <div>
              <label className="text-[10px] text-[var(--cds-support-warning)] uppercase block mb-1">{t('builder.field_note_text', 'Note Text')}</label>
              <textarea
                rows={4}
                value={selectedNode.noteText || ''}
                onChange={e => onUpdateNode({ noteText: e.target.value })}
                placeholder={t('builder.placeholder_note_text', 'Add workflow notes here')}
                className="w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-xs text-[var(--cds-text-primary)] font-sans resize-y"
              />
            </div>
          )}

          {/* Set as Initial Step Toggle — meaningless for a canvas-only annotation:
              notes are excluded from the generated scenario's steps, so buildScenarioObject
              never sees this flag. */}
          {selectedNode.type !== 'note' && (
            <div className="p-2 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] flex items-center justify-between">
              <span className="text-xs text-[var(--cds-text-secondary)]">{t('builder.is_initial_step', 'Is Initial Step (Start)')}</span>
              <input
                type="checkbox"
                checked={!!selectedNode.isInitial}
                onChange={e => onSetInitialNode(selectedNode.id, e.target.checked)}
                className="accent-[var(--cds-interactive)]"
              />
            </div>
          )}

          {/* Outgoing Transitions — hidden for notes: buildScenarioObject skips note nodes
              entirely, so any branches configured here would silently have no effect. */}
          {selectedNode.type !== 'note' && (
          <div className="space-y-1.5 pt-2 border-t border-[var(--cds-border-subtle)]">
            <div className="flex items-center justify-between">
              <span className="text-[10px] text-[var(--cds-interactive)] uppercase font-semibold">{t('builder.outgoing_branches', 'Outgoing Branches')}</span>
              <button
                onClick={() => {
                  const tr = selectedNode.transitions || [];
                  onUpdateNode({
                    transitions: [...tr, { target_step_id: 'END', probability: 0.5 }]
                  });
                }}
                className="text-[var(--cds-interactive)] text-[10px] hover:underline"
              >
                {t('builder.add_branch', '+ Add Branch')}
              </button>
            </div>

            {(selectedNode.transitions || []).map((tr, trIdx) => (
              <div key={trIdx} className="p-2 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] space-y-1.5">
                <div className="flex items-center justify-between">
                  <select 
                    value={tr.target_step_id} 
                    onChange={e => {
                      const updated = [...(selectedNode.transitions || [])];
                      updated[trIdx] = { ...updated[trIdx], target_step_id: e.target.value };
                      onUpdateNode({ transitions: updated });
                    }}
                    className="bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] text-[11px] px-1 py-0.5 text-[var(--cds-text-primary)] w-32"
                  >
                    <option value="END">-- END --</option>
                    {nodes.filter(n => n.id !== selectedNode.id).map(n => (
                      <option key={n.id} value={n.id}>{n.name} ({n.id})</option>
                    ))}
                  </select>
                  <button 
                    onClick={() => {
                      const updated = (selectedNode.transitions || []).filter((_, i) => i !== trIdx);
                      onUpdateNode({ transitions: updated });
                    }}
                    className="text-[var(--cds-support-error)] text-[10px]"
                  >
                    {t('builder.remove_branch', 'Remove')}
                  </button>
                </div>

                <div className="flex items-center space-x-2">
                  <span className="text-[10px] text-[var(--cds-text-helper)]">{t('builder.weight_label', 'Weight:')}</span>
                  <input 
                    type="range" 
                    min="0" 
                    max="1" 
                    step="0.05"
                    value={tr.probability || 1.0}
                    onChange={e => {
                      const updated = [...(selectedNode.transitions || [])];
                      updated[trIdx] = { ...updated[trIdx], probability: parseFloat(e.target.value) };
                      onUpdateNode({ transitions: updated });
                    }}
                    className="w-24 accent-[var(--cds-interactive)]"
                  />
                  <span className="text-[11px] text-[var(--cds-text-primary)]">{((tr.probability || 1.0) * 100).toFixed(0)}%</span>
                </div>
              </div>
            ))}
          </div>
          )}

        </div>
      ) : (
        <div className="p-4 text-center text-[var(--cds-text-helper)] flex-1 flex items-center justify-center">
          {t('builder.inspector_empty_hint', 'Click any step card on the canvas to configure parameters.')}
        </div>
      )}
    </div>
  );
}
