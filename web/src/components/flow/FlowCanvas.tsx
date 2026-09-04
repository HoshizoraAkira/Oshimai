import React, { RefObject } from 'react';
import FlowNodeCard from './FlowNodeCard';
import { useTranslation } from '../../context/I18nContext';
import { FlowNode } from '../../types/models';

export interface FlowCanvasProps {
  nodes: FlowNode[];
  selectedNodeId: string | null;
  connectingFrom: string | null;
  zoom: number;
  canvasRef: RefObject<HTMLDivElement>;
  onMouseMove: (e: React.MouseEvent) => void;
  onMouseUp: () => void;
  onMouseDownNode: (e: React.MouseEvent, node: FlowNode) => void;
  onSelectNode: (id: string) => void;
  onDeleteNode: (id: string) => void;
  onStartConnect: (id: string) => void;
  onCompleteConnect: (id: string) => void;
}

export default function FlowCanvas({
  nodes,
  selectedNodeId,
  connectingFrom,
  zoom,
  canvasRef,
  onMouseMove,
  onMouseUp,
  onMouseDownNode,
  onSelectNode,
  onDeleteNode,
  onStartConnect,
  onCompleteConnect,
}: FlowCanvasProps) {
  const { t } = useTranslation();

  return (
    <div 
      ref={canvasRef}
      onMouseMove={onMouseMove}
      onMouseUp={onMouseUp}
      className="relative flex-1 w-full bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] overflow-auto flow-grid-bg select-none"
    >
      <div style={{ transform: `scale(${zoom})`, transformOrigin: 'top left', minWidth: '2400px', minHeight: '1600px', position: 'relative' }}>
        
        {/* SVG Connecting Flow Lines / Arrows */}
        <svg className="absolute inset-0 w-[2400px] h-[1600px] pointer-events-none z-0">
          <defs>
            <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--cds-interactive)" />
            </marker>
            <marker id="arrowhead-gray" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--cds-border-strong)" />
            </marker>
          </defs>

          {nodes.map(sourceNode => {
            return (sourceNode.transitions || []).map((trans, tIdx) => {
              const targetNode = nodes.find(n => n.id === trans.target_step_id);
              if (!targetNode) return null;

              const startX = sourceNode.x + 320;
              const startY = sourceNode.y + 65;
              const endX = targetNode.x;
              const endY = targetNode.y + 65;

              const dx = Math.max(40, Math.abs(endX - startX) * 0.4);
              const pathD = `M ${startX} ${startY} C ${startX + dx} ${startY}, ${endX - dx} ${endY}, ${endX} ${endY}`;
              const midX = (startX + endX) / 2;
              const midY = (startY + endY) / 2;

              const pctText = trans.probability ? `${(trans.probability * 100).toFixed(0)}%` : '100%';

              return (
                <g key={`${sourceNode.id}-${trans.target_step_id}-${tIdx}`}>
                  <path 
                    d={pathD} 
                    fill="none" 
                    stroke={sourceNode.id === selectedNodeId ? 'var(--cds-interactive)' : 'var(--cds-layer-03)'} 
                    strokeWidth={sourceNode.id === selectedNodeId ? '2.5' : '1.5'}
                    strokeDasharray={trans.condition ? '4,4' : 'none'}
                    markerEnd={sourceNode.id === selectedNodeId ? 'url(#arrowhead)' : 'url(#arrowhead-gray)'}
                  />
                  <rect 
                    x={midX - 18} 
                    y={midY - 10} 
                    width="36" 
                    height="18" 
                    rx="2" 
                    fill="var(--cds-layer-01)" 
                    stroke="var(--cds-border-subtle)"
                  />
                  <text 
                    x={midX} 
                    y={midY + 3} 
                    textAnchor="middle" 
                    fill="var(--cds-text-primary)" 
                    fontSize="10" 
                    fontFamily="IBM Plex Mono"
                  >
                    {pctText}
                  </text>
                </g>
              );
            });
          })}
        </svg>

        {/* Draggable Step Cards */}
        {nodes.map(node => (
          <FlowNodeCard
            key={node.id}
            node={node}
            isSelected={node.id === selectedNodeId}
            isConnectingFrom={connectingFrom === node.id}
            onMouseDown={(e) => onMouseDownNode(e, node)}
            onClick={() => {
              if (connectingFrom) onCompleteConnect(node.id);
              else onSelectNode(node.id);
            }}
            onDelete={onDeleteNode}
            onStartConnect={onStartConnect}
          />
        ))}

        {/* Terminal END Indicator Node */}
        <div 
          style={{ left: '2280px', top: '160px' }}
          className="absolute w-28 p-3 bg-[var(--cds-background)] border-2 border-dashed border-[var(--cds-layer-03)] text-center font-mono text-xs text-[var(--cds-text-helper)]"
        >
          <div className="text-[10px] text-[var(--cds-text-primary)] font-bold">{t('builder.terminal_label', 'TERMINAL')}</div>
          <div className="text-sm font-bold text-[var(--cds-support-success)] mt-0.5">END</div>
        </div>

      </div>
    </div>
  );
}
