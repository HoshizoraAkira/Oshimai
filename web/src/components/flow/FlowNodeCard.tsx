import React from 'react';
import {
  TrashCan, Time, Notebook, Password, Security, ArrowRight
} from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { FlowNode } from '../../types/models';

export function getMethodBadge(m?: string): string {
  switch (m) {
    case 'POST': return 'bg-[color:rgb(36_161_72_/_0.16)] text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]';
    case 'GET': return 'bg-[color:rgb(15_98_254_/_0.16)] text-[var(--cds-link)] border-[var(--cds-interactive)]';
    case 'PUT': return 'bg-[color:rgb(241_194_27_/_0.16)] text-[var(--cds-support-warning)] border-[var(--cds-support-warning)]';
    case 'DELETE': return 'bg-[color:rgb(218_30_40_/_0.16)] text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]';
    default: return 'bg-[var(--cds-layer-02)] text-[var(--cds-text-secondary)] border-[var(--cds-layer-03)]';
  }
}

export interface FlowNodeCardProps {
  node: FlowNode;
  isSelected: boolean;
  isConnectingFrom: boolean;
  onMouseDown: (e: React.MouseEvent) => void;
  onClick: () => void;
  onDelete: (id: string) => void;
  onStartConnect: (id: string) => void;
}

export default function FlowNodeCard({
  node,
  isSelected,
  isConnectingFrom,
  onMouseDown,
  onClick,
  onDelete,
  onStartConnect,
}: FlowNodeCardProps) {
  const { t } = useTranslation();

  if (node.type === 'note') {
    return (
      <div
        onMouseDown={onMouseDown}
        onClick={onClick}
        style={{ left: `${node.x}px`, top: `${node.y}px` }}
        className={`absolute w-72 p-3 bg-[color:rgb(241_194_27_/_0.10)] border border-[var(--cds-support-warning)] text-[var(--cds-text-primary)] cursor-move z-10 ${
          isSelected ? 'ring-2 ring-[var(--cds-support-warning)]' : ''
        }`}
      >
        <div className="flex items-center justify-between pb-1 border-b border-[color:rgb(241_194_27_/_0.3)]">
          <span className="text-[10px] font-bold text-[var(--cds-support-warning)] flex items-center gap-1">
            <Notebook size={12} /> {t('builder.note_badge', 'NOTE')}
          </span>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onDelete(node.id);
            }}
            className="text-[var(--cds-text-helper)] hover:text-[var(--cds-support-error)]"
          >
            <TrashCan size={12} />
          </button>
        </div>
        <p className="text-xs mt-2 font-sans text-[var(--cds-text-primary)] leading-relaxed">
          {node.noteText || t('builder.note_default_text', 'Scenario architecture note.')}
        </p>
      </div>
    );
  }

  if (node.type === 'delay') {
    return (
      <div
        onMouseDown={onMouseDown}
        onClick={onClick}
        style={{ left: `${node.x}px`, top: `${node.y}px` }}
        className={`absolute w-80 bg-[var(--cds-layer-01)] border transition-shadow cursor-move z-10 ${
          isSelected
            ? 'border-[var(--cds-support-novel)] shadow-[0_0_16px_rgba(190,149,255,0.4)]'
            : 'border-[var(--cds-border-subtle)]'
        }`}
      >
        <div className="p-2.5 bg-[var(--cds-layer-hover-01)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between">
          <span className="text-[10px] font-bold px-1.5 py-0.5 border border-[var(--cds-support-novel)] bg-[color:rgb(165_110_255_/_0.16)] text-[var(--cds-support-novel-text)] flex items-center gap-1">
            <Time size={12} /> {t('builder.think_time_badge', 'THINK TIME')}
          </span>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onDelete(node.id);
            }}
            className="text-[var(--cds-text-helper)] hover:text-[var(--cds-support-error)]"
          >
            <TrashCan size={12} />
          </button>
        </div>
        <div className="p-3 space-y-1">
          <div className="text-xs text-[var(--cds-text-primary)] font-semibold">{node.name}</div>
          <div className="text-[11px] text-[var(--cds-support-novel-text)]">{t('builder.duration_label', 'Duration: %s ms', [node.delayMs || 1000])}</div>
          <div className="pt-1 flex items-center justify-between text-[10px] text-[var(--cds-text-helper)]">
            <span>{t('builder.branches_label', 'Branches: %s', [node.transitions?.length || 0])}</span>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onStartConnect(node.id);
              }}
              className="text-[var(--cds-interactive)] hover:underline flex items-center gap-0.5"
            >
              {t('builder.connect_label', 'Connect')} <ArrowRight size={12} />
            </button>
          </div>
        </div>
        <div 
          onClick={(e) => {
            e.stopPropagation();
            onStartConnect(node.id);
          }}
          className="absolute -right-2 top-1/2 -translate-y-1/2 w-4 h-4 rounded-full bg-[var(--cds-support-novel)] border-2 border-[var(--cds-background)] cursor-crosshair"
        />
      </div>
    );
  }

  // Default HTTP / Decision Card
  return (
    <div
      onMouseDown={onMouseDown}
      onClick={onClick}
      style={{ left: `${node.x}px`, top: `${node.y}px` }}
      className={`absolute w-80 bg-[var(--cds-layer-01)] border transition-shadow cursor-move z-10 ${
        isSelected 
          ? 'border-[var(--cds-interactive)] shadow-[0_0_16px_rgba(15,98,254,0.45)]' 
          : 'border-[var(--cds-border-subtle)] hover:border-[var(--cds-border-strong)]'
      } ${isConnectingFrom ? 'ring-2 ring-[var(--cds-support-warning)]' : ''}`}
    >
      {/* Card Header */}
      <div className="p-3 bg-[var(--cds-layer-hover-01)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <span className={`text-[10px] font-mono font-bold px-1.5 py-0.5 border ${getMethodBadge(node.method || 'GET')}`}>
            {node.method || 'GET'}
          </span>
          <span className="font-semibold text-xs text-[var(--cds-text-primary)] truncate max-w-[140px]" title={node.name}>
            {node.name}
          </span>
        </div>
        <div className="flex items-center space-x-1">
          {node.isInitial && (
            <span className="text-[9px] font-mono bg-[color:rgb(36_161_72_/_0.16)] text-[var(--cds-support-success-text)] px-1.5 py-0.5 border border-[var(--cds-support-success)] font-bold">
              {t('builder.start_badge', 'START')}
            </span>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              onDelete(node.id);
            }}
            className="text-[var(--cds-text-helper)] hover:text-[var(--cds-support-error)] p-1"
            title={t('builder.title_delete_node', 'Delete node')}
          >
            <TrashCan size={14} />
          </button>
        </div>
      </div>

      {/* Card Body */}
      <div className="p-3.5 space-y-2 text-xs font-mono">
        <div className="text-[11px] text-[var(--cds-text-helper)] truncate">
          ID: <span className="text-[var(--cds-text-primary)] font-semibold">{node.id}</span>
        </div>
        <div className="bg-[var(--cds-background)] p-1.5 text-[11px] text-[var(--cds-support-success)] truncate border border-[var(--cds-border-subtle)]">
          {node.path || '/'}
        </div>

        {/* Extra indicators: Assertions / Auth */}
        <div className="flex items-center space-x-2 text-[10px]">
          {node.authBearer && (
            <span className="text-[var(--cds-support-warning)] flex items-center gap-0.5">
              <Password size={12} /> {t('builder.auth_badge', 'Auth')}
            </span>
          )}
          {node.assertions && node.assertions.length > 0 && (
            <span className="text-[var(--cds-support-success-text)] flex items-center gap-0.5">
              <Security size={12} /> {t('builder.assert_badge_suffix', '%s Assert', [node.assertions.length])}
            </span>
          )}
        </div>

        {/* Transitions summary */}
        <div className="pt-1.5 border-t border-[var(--cds-border-subtle)] flex items-center justify-between text-[11px] text-[var(--cds-text-helper)]">
          <span>{t('builder.branches_label', 'Branches: %s', [node.transitions?.length || 0])}</span>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onStartConnect(node.id);
            }}
            className="text-[var(--cds-interactive)] hover:underline text-[11px] flex items-center gap-1 font-semibold"
          >
            <span>{t('builder.connect_label', 'Connect')}</span>
            <ArrowRight size={12} />
          </button>
        </div>
      </div>

      {/* Output connector port dot */}
      <div
        onClick={(e) => {
          e.stopPropagation();
          onStartConnect(node.id);
        }}
        className="absolute -right-2.5 top-1/2 -translate-y-1/2 w-5 h-5 rounded-full bg-[var(--cds-interactive)] border-2 border-[var(--cds-background)] cursor-crosshair hover:scale-125 transition-transform flex items-center justify-center"
        title={t('builder.title_connect_node', 'Click to connect to another node')}
      >
        <div className="w-1.5 h-1.5 bg-white rounded-full"></div>
      </div>
    </div>
  );
}
