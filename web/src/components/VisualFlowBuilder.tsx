import React, { useState, useRef, useEffect } from 'react';
import { useTranslation } from '../context/I18nContext';
import { SUBTAB_TO_VIEW_MODE, VIEW_MODE_TO_SUBTAB } from '../constants/flowNodes';
import { buildScenarioObject } from '../utils/scenarioBuilder';
import { scenarioLibrary } from '../services/scenarioLibrary';
import { FlowNode, SavedScenario, ToastType } from '../types/models';

import FlowToolbar from './flow/FlowToolbar';
import FlowCanvas from './flow/FlowCanvas';
import NodeInspector from './flow/NodeInspector';
import SpecImporter from './flow/SpecImporter';
import CodeView from './flow/CodeView';
import ScenarioListView from './flow/ScenarioListView';

export interface VisualFlowBuilderProps {
  onSendToLauncher: (yaml: string, baseUrl?: string) => void;
  showToast: (msg: string, type?: ToastType) => void;
  activeSubTab?: string;
  onSelectSubTab?: (subTab: string) => void;
}

const PRESET_NODES: Record<'ecommerce' | 'auth', { nodes: FlowNode[]; scenarioId: string }> = {
  ecommerce: {
    scenarioId: 'ecommerce_user_funnel',
    nodes: [
      { id: 'browse_catalog', name: '1. Browse Catalog', type: 'http', method: 'GET', path: '/api/v1/products', x: 80, y: 140, isInitial: true, transitions: [{ target_step_id: 'product_detail', probability: 0.8 }, { target_step_id: 'END', probability: 0.2 }] },
      { id: 'product_detail', name: '2. Product Detail', type: 'http', method: 'GET', path: '/api/v1/products/item-123', x: 480, y: 140, transitions: [{ target_step_id: 'add_to_cart', probability: 0.6 }, { target_step_id: 'think_browse', probability: 0.4 }] },
      { id: 'think_browse', name: 'Think Pause', type: 'delay', delayMs: 2500, x: 480, y: 380, transitions: [{ target_step_id: 'browse_catalog', probability: 1.0 }] },
      { id: 'add_to_cart', name: '3. Add to Cart', type: 'http', method: 'POST', path: '/api/v1/cart', body: '{"item_id":"item-123","qty":1}', x: 880, y: 140, transitions: [{ target_step_id: 'checkout', probability: 0.9 }] },
      { id: 'checkout', name: '4. Checkout & Pay', type: 'http', method: 'POST', path: '/api/v1/orders', body: '{"payment_method":"gopay"}', x: 1280, y: 140, transitions: [{ target_step_id: 'END', probability: 1.0 }] },
    ],
  },
  auth: {
    scenarioId: 'auth_lifecycle_journey',
    nodes: [
      { id: 'login', name: 'Login Step', type: 'http', method: 'POST', path: '/auth/login', body: '{"username":"demo","password":"secret"}', x: 80, y: 160, isInitial: true, transitions: [{ target_step_id: 'get_profile', probability: 1.0 }] },
      { id: 'get_profile', name: 'Get Profile', type: 'http', method: 'GET', path: '/user/profile', authBearer: '{{auth_token}}', x: 480, y: 160, transitions: [{ target_step_id: 'logout', probability: 0.9 }] },
      { id: 'logout', name: 'Logout', type: 'http', method: 'POST', path: '/auth/logout', x: 880, y: 160, transitions: [{ target_step_id: 'END', probability: 1.0 }] },
    ],
  },
};

export default function VisualFlowBuilder({
  onSendToLauncher,
  showToast,
  activeSubTab,
  onSelectSubTab
}: VisualFlowBuilderProps) {
  const { t } = useTranslation();

  // Landing page (a table of every scenario) vs. the flow editor for one of them.
  // Re-entering the Scenario Builder tab always unmounts this component (see
  // App.tsx), so it's fine for this to reset to 'list' every time — that IS the
  // desired entry point now instead of always dropping back into the last canvas.
  const [mode, setMode] = useState<'list' | 'editor'>('list');
  // The saved-template id currently open for editing, or null for an unsaved/new
  // scenario (a fresh one, or a preset opened for editing) — determines whether
  // "Save" overwrites that template in place or prompts to create a new one.
  const [currentSavedId, setCurrentSavedId] = useState<string | null>(null);

  const [nodes, setNodes] = useState<FlowNode[]>([]);
  const [history, setHistory] = useState<FlowNode[][]>([[]]);
  const [historyIdx, setHistoryIdx] = useState(0);

  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [internalViewMode, setInternalViewMode] = useState<'visual' | 'code' | 'importer'>('visual');
  // `activeSubTab` is shared app-wide across every top-level tab (see App.tsx), so it can
  // arrive here holding another tab's subtab id (e.g. Launcher's 'spec') that isn't one of
  // this builder's own view modes. Falling through to that raw value used to render nothing
  // at all — fall back to the builder's own last-known view mode instead, same guard pattern
  // OpsConsole uses for its subtabs.
  const mappedViewMode = activeSubTab ? (SUBTAB_TO_VIEW_MODE as any)[activeSubTab] : undefined;
  const viewMode: 'visual' | 'code' | 'importer' = onSelectSubTab
    ? (mappedViewMode || internalViewMode)
    : internalViewMode;

  const setViewMode = (newMode: 'visual' | 'code' | 'importer') => {
    setInternalViewMode(newMode);
    if (onSelectSubTab) onSelectSubTab((VIEW_MODE_TO_SUBTAB as any)[newMode] || newMode);
  };

  const [scenarioId, setScenarioId] = useState('user_journey_flow');
  const [baseUrl, setBaseUrl] = useState('https://httpbin.org');

  // User-authored scenario templates (saved to browser localStorage)
  const [savedScenarios, setSavedScenarios] = useState<SavedScenario[]>(() => scenarioLibrary.list());
  const refreshSavedScenarios = () => setSavedScenarios(scenarioLibrary.list());

  // Canvas Interaction State
  const [zoom, setZoom] = useState(1.0);
  const [draggingNode, setDraggingNode] = useState<FlowNode | null>(null);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [connectingFrom, setConnectingFrom] = useState<string | null>(null);
  const [rawYaml, setRawYaml] = useState('');

  const canvasRef = useRef<HTMLDivElement>(null);

  // Sync to YAML/JSON
  useEffect(() => {
    const scenarioObj = buildScenarioObject(nodes, scenarioId, baseUrl);
    setRawYaml(JSON.stringify(scenarioObj, null, 2));
  }, [nodes, scenarioId, baseUrl]);

  // Push state to Undo/Redo stack
  const updateNodesWithHistory = (newNodes: FlowNode[]) => {
    const newHist = history.slice(0, historyIdx + 1);
    newHist.push(newNodes);
    setHistory(newHist);
    setHistoryIdx(newHist.length - 1);
    setNodes(newNodes);
  };

  const handleUndo = () => {
    if (historyIdx > 0) {
      const targetIdx = historyIdx - 1;
      setHistoryIdx(targetIdx);
      setNodes(history[targetIdx]);
      showToast(t('builder.toast_undo', 'Action undone.'), 'info');
    }
  };

  const handleRedo = () => {
    if (historyIdx < history.length - 1) {
      const targetIdx = historyIdx + 1;
      setHistoryIdx(targetIdx);
      setNodes(history[targetIdx]);
      showToast(t('builder.toast_redo', 'Action redone.'), 'info');
    }
  };

  // Keyboard shortcut listener (Ctrl+Z, Ctrl+Y)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'z') {
        e.preventDefault();
        if (e.shiftKey) handleRedo();
        else handleUndo();
      } else if ((e.metaKey || e.ctrlKey) && e.key === 'y') {
        e.preventDefault();
        handleRedo();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [historyIdx, history]);

  // Dragging handlers
  const handleMouseDownNode = (e: React.MouseEvent, node: FlowNode) => {
    e.stopPropagation();
    setSelectedNodeId(node.id);
    setDraggingNode(node);
    setDragOffset({
      x: e.clientX - node.x,
      y: e.clientY - node.y
    });
  };

  const handleMouseMoveCanvas = (e: React.MouseEvent) => {
    if (!draggingNode) return;
    const newX = Math.max(0, e.clientX - dragOffset.x);
    const newY = Math.max(0, e.clientY - dragOffset.y);
    setNodes(prev => prev.map(n => n.id === draggingNode.id ? { ...n, x: newX, y: newY } : n));
  };

  const handleMouseUpCanvas = () => {
    if (draggingNode) {
      updateNodesWithHistory(nodes);
      setDraggingNode(null);
    }
  };

  // Node editing handlers
  const updateSelectedNode = (fields: Partial<FlowNode>) => {
    const updated = nodes.map(n => n.id === selectedNodeId ? { ...n, ...fields } : n);
    updateNodesWithHistory(updated);
  };

  const handleSetInitialNode = (nodeId: string, checked: boolean) => {
    const updated = nodes.map(n => ({
      ...n,
      isInitial: n.id === nodeId ? checked : false
    }));
    updateNodesWithHistory(updated);
  };

  const handleAddNode = (type: 'http' | 'delay' | 'decision' | 'note', method: string = 'GET') => {
    const count = nodes.filter(n => n.type === type).length + 1;
    const newId = `${type}_${count}_${Date.now().toString().slice(-4)}`;
    const newNode: FlowNode = {
      id: newId,
      name: `${type.toUpperCase()} Step ${count}`,
      type,
      method: type === 'http' ? method : undefined,
      path: type === 'http' ? `/api/v1/step_${count}` : undefined,
      x: 350 + (nodes.length % 3) * 60,
      y: 180 + (nodes.length % 3) * 60,
      delayMs: type === 'delay' ? 1000 : undefined,
      noteText: type === 'note' ? 'Add workflow notes here' : undefined,
      transitions: type !== 'note' ? [{ target_step_id: 'END', probability: 1.0 }] : [],
    };

    updateNodesWithHistory([...nodes, newNode]);
    setSelectedNodeId(newId);
    showToast(t('builder.toast_node_added', 'Added %s node: %s', [type.toUpperCase(), newId]), 'success');
  };

  const handleDuplicateNode = (nodeId: string) => {
    const target = nodes.find(n => n.id === nodeId);
    if (!target) return;
    const clonedId = `${target.id}_copy_${Date.now().toString().slice(-4)}`;
    const cloned: FlowNode = {
      ...target,
      id: clonedId,
      name: `${target.name} (Copy)`,
      x: target.x + 40,
      y: target.y + 40,
      isInitial: false,
    };
    updateNodesWithHistory([...nodes, cloned]);
    setSelectedNodeId(clonedId);
    showToast(t('builder.toast_cloned', 'Cloned step: %s', [clonedId]), 'success');
  };

  const handleDeleteNode = (nodeId: string) => {
    const updated = nodes
      .filter(n => n.id !== nodeId)
      .map(n => ({
        ...n,
        transitions: (n.transitions || []).filter(t => t.target_step_id !== nodeId)
      }));
    updateNodesWithHistory(updated);
    if (selectedNodeId === nodeId) setSelectedNodeId(null);
    showToast(t('builder.toast_node_deleted', 'Deleted step %s.', [nodeId]), 'info');
  };

  // Connection handlers
  const handleStartConnect = (sourceId: string) => {
    setConnectingFrom(sourceId);
    showToast(t('builder.toast_connect_from', 'Select target node to connect from %s...', [sourceId]), 'info');
  };

  const handleCompleteConnect = (targetId: string) => {
    if (!connectingFrom) return;
    if (connectingFrom === targetId) {
      showToast(t('builder.toast_connect_self', 'Cannot connect node to itself.'), 'warning');
      setConnectingFrom(null);
      return;
    }

    const updated = nodes.map(n => {
      if (n.id !== connectingFrom) return n;
      const transitions = n.transitions || [];
      if (transitions.some(t => t.target_step_id === targetId)) {
        return n;
      }
      return {
        ...n,
        transitions: [...transitions, { target_step_id: targetId, probability: 1.0 }]
      };
    });

    updateNodesWithHistory(updated);
    showToast(t('builder.toast_connected', 'Connected %s → %s', [connectingFrom, targetId]), 'success');
    setConnectingFrom(null);
  };

  // Validation
  //
  // getFlowBlockingError() covers only the conditions the backend actually rejects a
  // scenario for (empty flow, no initial_step_id — see pkg/vusession/parser.go) so it can
  // gate "Execute in Launcher" without inventing stricter client-side-only rules. The
  // remaining checks (multiple START steps, dead ends) are advisory and stay inside
  // handleValidateFlow as warnings only.
  const getFlowBlockingError = (): string | null => {
    const executableNodes = nodes.filter(n => n.type !== 'note');
    if (executableNodes.length === 0) {
      return t('builder.toast_empty_flow', 'Flow error: Add at least one step before executing.');
    }
    if (!nodes.some(n => n.isInitial)) {
      return t('builder.toast_no_start', 'Flow error: No initial START step designated.');
    }
    return null;
  };

  const handleValidateFlow = () => {
    const blockingError = getFlowBlockingError();
    if (blockingError) {
      showToast(blockingError, 'error');
      return;
    }

    const initialNodes = nodes.filter(n => n.isInitial);
    if (initialNodes.length > 1) {
      showToast(t('builder.toast_multi_start', 'Flow warning: Multiple START steps defined.'), 'warning');
      return;
    }

    const deadEnds = nodes.filter(n => n.type !== 'note' && (!n.transitions || n.transitions.length === 0));
    if (deadEnds.length > 0) {
      showToast(t('builder.toast_dead_end', 'Flow warning: Step "%s" has no outgoing transitions or END terminator.', [deadEnds[0].id]), 'warning');
      return;
    }

    showToast(t('builder.toast_validate_ok', 'Flow verification PASSED: State machine is complete and sound.'), 'success');
  };

  const handleSendToLauncher = () => {
    const blockingError = getFlowBlockingError();
    if (blockingError) {
      showToast(blockingError, 'error');
      return;
    }
    onSendToLauncher(rawYaml, baseUrl);
  };

  // Auto Layout
  const handleAutoLayout = () => {
    const updated = nodes.map((n, idx) => ({
      ...n,
      x: 80 + (idx % 3) * 440,
      y: 120 + Math.floor(idx / 3) * 220,
    }));
    updateNodesWithHistory(updated);
    showToast(t('builder.toast_autolayout', 'Auto-layout arranged.'), 'info');
  };

  // Opens the editor for a given scenario, replacing whatever was previously
  // being edited (including its undo/redo history — these are different
  // scenarios, not steps within the same one).
  const loadIntoEditor = (newNodes: FlowNode[], newScenarioId: string, newBaseUrl: string, savedId: string | null) => {
    setNodes(newNodes);
    setHistory([newNodes]);
    setHistoryIdx(0);
    setScenarioId(newScenarioId);
    setBaseUrl(newBaseUrl);
    setCurrentSavedId(savedId);
    setSelectedNodeId(newNodes.find(n => n.isInitial)?.id || newNodes[0]?.id || null);
    setInternalViewMode('visual');
    setMode('editor');
  };

  const handleCreateNew = () => {
    loadIntoEditor([], `new_scenario_${Date.now().toString(36)}`, 'https://httpbin.org', null);
    showToast(t('builder.toast_new_scenario', 'Started a new blank scenario.'), 'info');
  };

  // Opening a built-in preset for editing forks it — presets themselves are
  // hardcoded and not stored in the library, so "Save" always creates a new entry.
  const handleEditPreset = (presetName: 'ecommerce' | 'auth') => {
    const preset = PRESET_NODES[presetName];
    loadIntoEditor(preset.nodes, preset.scenarioId, 'https://httpbin.org', null);
    const presetTitle = presetName === 'ecommerce' ? t('builder.preset_ecommerce_title') : t('builder.preset_auth_title');
    showToast(t('builder.toast_editing_preset', 'Editing "%s" preset.', [presetTitle]), 'info');
  };

  const handleEditSaved = (s: SavedScenario) => {
    loadIntoEditor(s.nodes, s.scenarioId, s.baseUrl, s.id);
    showToast(t('builder.toast_editing_saved', 'Editing "%s".', [s.name]), 'success');
  };

  const handleBackToList = () => {
    setMode('list');
    refreshSavedScenarios();
  };

  const handleSave = () => {
    if (currentSavedId) {
      scenarioLibrary.update(currentSavedId, { scenarioId, baseUrl, nodes });
      refreshSavedScenarios();
      showToast(t('builder.toast_scenario_updated', 'Scenario updated.'), 'success');
    } else {
      const name = window.prompt(t('builder.prompt_name_scenario', 'Name this scenario:'), scenarioId);
      if (!name || !name.trim()) return;
      const saved = scenarioLibrary.save(name.trim(), scenarioId, baseUrl, nodes);
      setCurrentSavedId(saved.id);
      refreshSavedScenarios();
      showToast(t('builder.toast_saved_scenario', 'Saved scenario "%s".', [name.trim()]), 'success');
    }
  };

  const handleRenameSavedScenario = (s: SavedScenario) => {
    const name = window.prompt(t('builder.prompt_rename_scenario', 'Rename scenario template:'), s.name);
    if (!name || !name.trim() || name.trim() === s.name) return;
    scenarioLibrary.rename(s.id, name.trim());
    refreshSavedScenarios();
  };

  const handleDeleteSavedScenario = (s: SavedScenario) => {
    if (!window.confirm(t('builder.confirm_delete_scenario', 'Delete scenario template "%s"? This cannot be undone.', [s.name]))) return;
    scenarioLibrary.remove(s.id);
    refreshSavedScenarios();
    showToast(t('builder.toast_deleted_scenario', 'Deleted scenario template "%s".', [s.name]), 'info');
  };

  const handleImportSuccess = (newNodes: FlowNode[], newScenarioId: string) => {
    updateNodesWithHistory(newNodes);
    setScenarioId(newScenarioId);
    setViewMode('visual');
  };

  const handleSyncCodeToNodes = (newNodes: FlowNode[], meta?: { scenarioId?: string; baseUrl?: string }) => {
    updateNodesWithHistory(newNodes);
    if (meta?.scenarioId) setScenarioId(meta.scenarioId);
    if (meta?.baseUrl) setBaseUrl(meta.baseUrl);
  };

  const selectedNode = nodes.find(n => n.id === selectedNodeId) || null;

  if (mode === 'list') {
    return (
      <ScenarioListView
        savedScenarios={savedScenarios}
        onCreateNew={handleCreateNew}
        onEditPreset={handleEditPreset}
        onEditSaved={handleEditSaved}
        onRenameSaved={handleRenameSavedScenario}
        onDeleteSaved={handleDeleteSavedScenario}
      />
    );
  }

  return (
    <div className="space-y-4">
      {/* Top Toolbar */}
      <FlowToolbar
        scenarioId={scenarioId}
        setScenarioId={setScenarioId}
        baseUrl={baseUrl}
        setBaseUrl={setBaseUrl}
        viewMode={viewMode}
        setViewMode={setViewMode}
        onSendToLauncher={handleSendToLauncher}
        sendToLauncherDisabled={!!getFlowBlockingError()}
        onBackToList={handleBackToList}
        onSave={handleSave}
        isEditingExisting={!!currentSavedId}
        onAddNode={handleAddNode}
        onUndo={handleUndo}
        onRedo={handleRedo}
        canUndo={historyIdx > 0}
        canRedo={historyIdx < history.length - 1}
        selectedNodeId={selectedNodeId}
        onDuplicateNode={handleDuplicateNode}
        onValidateFlow={handleValidateFlow}
        zoom={zoom}
        setZoom={setZoom}
        onAutoLayout={handleAutoLayout}
      />

      {/* VIEW 1: INTERACTIVE 2D VISUAL FLOW CANVAS */}
      {viewMode === 'visual' && (
        <div className="flex flex-col lg:flex-row gap-4 h-[calc(100vh-210px)] min-h-[720px]">
          <div className="flex-1 flex flex-col space-y-2 min-w-0 h-full">
            <FlowCanvas
              nodes={nodes}
              selectedNodeId={selectedNodeId}
              connectingFrom={connectingFrom}
              zoom={zoom}
              canvasRef={canvasRef}
              onMouseMove={handleMouseMoveCanvas}
              onMouseUp={handleMouseUpCanvas}
              onMouseDownNode={handleMouseDownNode}
              onSelectNode={setSelectedNodeId}
              onDeleteNode={handleDeleteNode}
              onStartConnect={handleStartConnect}
              onCompleteConnect={handleCompleteConnect}
            />
          </div>

          <NodeInspector
            selectedNode={selectedNode}
            nodes={nodes}
            onUpdateNode={updateSelectedNode}
            onSetInitialNode={handleSetInitialNode}
          />
        </div>
      )}

      {/* VIEW 2: RAW CODE */}
      {viewMode === 'code' && (
        <CodeView
          rawYaml={rawYaml}
          setRawYaml={setRawYaml}
          onSyncCodeToNodes={handleSyncCodeToNodes}
          showToast={showToast}
        />
      )}

      {/* VIEW 3: OPENAPI / OTEL IMPORTER */}
      {viewMode === 'importer' && (
        <SpecImporter
          baseUrl={baseUrl}
          onImportSuccess={handleImportSuccess}
          showToast={showToast}
        />
      )}
    </div>
  );
}
