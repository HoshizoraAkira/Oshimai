import React from 'react';
import { Add, Edit, Tag, TrashCan, Layers, Password } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';
import { SavedScenario } from '../../types/models';

export interface ScenarioListViewProps {
  savedScenarios: SavedScenario[];
  onCreateNew: () => void;
  onEditPreset: (presetName: 'ecommerce' | 'auth') => void;
  onEditSaved: (s: SavedScenario) => void;
  onRenameSaved: (s: SavedScenario) => void;
  onDeleteSaved: (s: SavedScenario) => void;
}

// Landing page for the Scenario Builder — a flat table of every scenario the
// operator can open (built-in presets, read-only, plus their own saved
// templates), instead of burying "my scenarios" inside one of the editor's
// view-mode tabs.
export default function ScenarioListView({
  savedScenarios,
  onCreateNew,
  onEditPreset,
  onEditSaved,
  onRenameSaved,
  onDeleteSaved,
}: ScenarioListViewProps) {
  const { t, formatTime } = useTranslation();

  const presetRows: Array<{ id: 'ecommerce' | 'auth'; icon: any; iconColor: string; name: string; desc: string }> = [
    {
      id: 'ecommerce',
      icon: Layers,
      iconColor: 'text-[var(--cds-interactive)]',
      name: t('builder.preset_ecommerce_title'),
      desc: t('builder.preset_ecommerce_desc'),
    },
    {
      id: 'auth',
      icon: Password,
      iconColor: 'text-[var(--cds-support-novel)]',
      name: t('builder.preset_auth_title'),
      desc: t('builder.preset_auth_desc'),
    },
  ];

  return (
    <div className="cds-tile">
      <div className="cds-tile-header">
        <div>
          <div className="cds-tile-eyebrow">{t('builder.presets', 'Scenario Library')}</div>
          <h3 className="cds-tile-title">{t('builder.scenario_list_title', 'Scenarios')}</h3>
        </div>
        <button type="button" onClick={onCreateNew} className="cds-btn-primary cds-btn-sm normal-case">
          <Add size={14} />
          <span>{t('builder.add_scenario', 'Add Scenario')}</span>
        </button>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left font-mono text-xs">
          <thead className="bg-[var(--cds-background)] text-[var(--cds-text-helper)] uppercase text-[10px] border-b border-[var(--cds-border-subtle)]">
            <tr>
              <th className="p-3">{t('common.name', 'Name')}</th>
              <th className="p-3">{t('builder.col_steps', 'Steps')}</th>
              <th className="p-3">{t('builder.col_type', 'Type')}</th>
              <th className="p-3">{t('builder.col_modified', 'Last Modified')}</th>
              <th className="p-3">{t('common.actions', 'Actions')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)]">
            {presetRows.map(p => {
              const Icon = p.icon;
              return (
                <tr key={p.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                  <td className="p-3 text-[var(--cds-text-primary)] font-semibold align-top">
                    <div className="flex items-center gap-2">
                      <Icon size={14} className={p.iconColor} />
                      <span>{p.name}</span>
                    </div>
                    <div className="text-[10px] text-[var(--cds-text-helper)] font-normal mt-0.5 max-w-md">{p.desc}</div>
                  </td>
                  <td className="p-3 align-top">—</td>
                  <td className="p-3 align-top"><span className="cds-badge-warning">{t('builder.type_preset', 'Preset')}</span></td>
                  <td className="p-3 align-top text-[var(--cds-text-helper)]">—</td>
                  <td className="p-3 align-top">
                    <button type="button" onClick={() => onEditPreset(p.id)} className="cds-btn-secondary cds-btn-sm normal-case" title={t('common.edit', 'Edit')}>
                      <Edit size={14} />
                      <span>{t('common.edit', 'Edit')}</span>
                    </button>
                  </td>
                </tr>
              );
            })}

            {savedScenarios.length === 0 ? (
              <tr>
                <td colSpan={5} className="p-4 text-center text-[var(--cds-text-helper)]">
                  {t('builder.no_saved_scenarios', 'No saved scenarios yet — build one and save it as a template.')}
                </td>
              </tr>
            ) : savedScenarios.map(s => (
              <tr key={s.id} className="hover:bg-[var(--cds-layer-hover-01)] transition-colors">
                <td className="p-3 text-[var(--cds-text-primary)] font-semibold">{s.name}</td>
                <td className="p-3">{t('builder.step_count', '%s steps', [s.nodes.length])}</td>
                <td className="p-3"><span className="cds-badge-info">{t('builder.type_custom', 'Custom')}</span></td>
                <td className="p-3 text-[var(--cds-text-helper)]">{formatTime(s.savedAt)}</td>
                <td className="p-3">
                  <div className="flex items-center gap-1">
                    <button type="button" onClick={() => onEditSaved(s)} className="cds-btn-ghost cds-btn-sm px-1.5" title={t('common.edit', 'Edit')}>
                      <Edit size={14} />
                    </button>
                    <button type="button" onClick={() => onRenameSaved(s)} className="cds-btn-ghost cds-btn-sm px-1.5" title={t('common.rename', 'Rename')}>
                      <Tag size={14} />
                    </button>
                    <button type="button" onClick={() => onDeleteSaved(s)} className="cds-btn-ghost cds-btn-sm px-1.5 text-[var(--cds-support-error)]" title={t('common.delete', 'Delete')}>
                      <TrashCan size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
