import React from 'react';
import {
  Play, Layers, RecentlyViewed, DataCenter,
  Help, Globe, Close, SidePanelClose, SidePanelOpen
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';

export interface SidebarNavProps {
  activeMenu: string;
  onSelectMenu: (menu: string) => void;
  onOpenGuide: () => void;
  isOpen?: boolean;
  onClose?: () => void;
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
}

export default function SidebarNav({
  activeMenu,
  onSelectMenu,
  onOpenGuide,
  isOpen = false,
  onClose,
  isCollapsed = false,
  onToggleCollapse,
}: SidebarNavProps) {
  const { t, lang, setLang, languages } = useTranslation();

  // Live Cockpit isn't a standalone destination — it only opens contextually
  // when a run starts from the Launcher, or when inspecting a past run from
  // History & Trends, so it's intentionally left out of the primary nav.
  const mainNavItems = [
    {
      id: 'launcher',
      label: t('nav.launcher', 'Launcher'),
      icon: Play,
      desc: t('nav.launcher_desc', 'Target, profile, chaos & execution'),
    },
    {
      id: 'builder',
      label: t('nav.builder', 'Scenario Builder'),
      icon: Layers,
      desc: t('nav.builder_desc', 'Visual flow authoring & spec import'),
    },
    {
      id: 'history',
      label: t('nav.history', 'History & Trends'),
      icon: RecentlyViewed,
      desc: t('nav.history_desc', 'Audit trail & baseline comparison'),
    },
    {
      id: 'ops',
      label: t('nav.ops', 'Ops Console'),
      icon: DataCenter,
      desc: t('nav.ops_desc', 'Multi-region, K8s chaos & compliance'),
    },
  ];

  // The collapse toggle only applies at the `lg` breakpoint where the sidebar
  // is static chrome — below it, the sidebar is an off-canvas drawer that
  // should always render fully expanded, so collapse-driven classes are
  // scoped with `lg:` and have no effect on mobile.
  const hideWhenCollapsed = isCollapsed ? 'lg:hidden' : '';

  return (
    <>
      {/* Mobile backdrop — closes the drawer on outside tap */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 lg:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        className={`w-64 ${isCollapsed ? 'lg:w-16' : 'lg:w-64'} bg-[var(--cds-background)] border-r border-[var(--cds-border-subtle)] flex flex-col shrink-0
                    fixed inset-y-0 left-0 z-50 h-screen select-none
                    transition-transform duration-200 ease-in-out lg:transition-[width]
                    ${isOpen ? 'translate-x-0' : '-translate-x-full'}
                    lg:translate-x-0 lg:static lg:sticky lg:top-0 lg:z-30`}
      >
      {/* Brand Header */}
      <div className={`h-12 border-b border-[var(--cds-border-subtle)] px-4 flex items-center justify-between shrink-0 ${isCollapsed ? 'lg:px-0 lg:justify-center' : ''}`}>
        <div className="flex items-center gap-2">
          <svg className="w-5 h-5 text-[var(--cds-interactive)] shrink-0" viewBox="0 0 32 32" fill="currentColor">
            <path d="M26 4h-6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2zm0 8h-6V6h6zm-14 6H6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2zm0 8H6v-6h6zm14-2h-6a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2zm0 8h-6v-6h6z" />
          </svg>
          <span className={`font-bold font-mono tracking-wider text-sm text-white ${hideWhenCollapsed}`}>OSHIMAI</span>
        </div>
        <div className="flex items-center gap-2">
          <div className={`hidden sm:flex items-center gap-1.5 px-2 py-0.5 bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] text-[10px] font-mono text-[var(--cds-support-success)] ${hideWhenCollapsed}`}>
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--cds-support-success)] pulse-dot-green"></span>
            <span>{t('admin.online', 'ONLINE')}</span>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-[var(--cds-text-helper)] hover:text-white lg:hidden"
            title={t('common.title_close_nav', 'Close navigation')}
          >
            <Close size={18} />
          </button>
        </div>
      </div>

      {/* Navigation Menus */}
      <nav className="flex-1 overflow-y-auto py-3 px-2 space-y-1 font-mono text-xs">
        {mainNavItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeMenu === item.id;

          return (
            <button
              key={item.id}
              type="button"
              onClick={() => { onSelectMenu(item.id); onClose && onClose(); }}
              title={isCollapsed ? item.label : undefined}
              className={`w-full flex items-center gap-3 px-3 py-2.5 text-left transition-colors ${isCollapsed ? 'lg:justify-center lg:px-0' : ''} ${
                isActive
                  ? 'bg-[var(--cds-layer-01)] text-white font-semibold border-l-2 border-[var(--cds-interactive)]'
                  : 'text-[var(--cds-text-secondary)] hover:bg-[var(--cds-surface-header)] hover:text-white'
              }`}
            >
              <Icon size={16} className={`shrink-0 ${isActive ? 'text-[var(--cds-interactive)]' : 'text-[var(--cds-text-helper)]'}`} />
              <div className={`flex-1 truncate ${hideWhenCollapsed}`}>
                <div className="truncate text-xs">{item.label}</div>
              </div>
            </button>
          );
        })}
      </nav>

      {/* Collapse / Expand Sidebar (desktop only — mobile drawer is always full width) */}
      {onToggleCollapse && (
        <button
          type="button"
          onClick={onToggleCollapse}
          className={`hidden lg:flex items-center gap-2 h-9 border-t border-[var(--cds-border-subtle)] text-[var(--cds-text-helper)] hover:text-white hover:bg-[var(--cds-surface-header)] text-[11px] font-mono shrink-0 ${isCollapsed ? 'justify-center' : 'px-3'}`}
          title={isCollapsed ? t('common.expand_sidebar', 'Expand sidebar') : t('common.title_collapse_sidebar', 'Collapse sidebar')}
        >
          {isCollapsed ? <SidePanelOpen size={16} /> : <SidePanelClose size={16} />}
          <span className={hideWhenCollapsed}>{t('common.collapse_sidebar', 'Collapse')}</span>
        </button>
      )}

      {/* Sidebar Footer: Language Switcher & Controls */}
      <div className={`p-3 border-t border-[var(--cds-border-subtle)] bg-[var(--cds-surface-deep)] space-y-2 ${isCollapsed ? 'lg:hidden' : ''}`}>
        <div className="flex items-center justify-between text-[11px] font-mono">
          <span className="text-[var(--cds-text-helper)] flex items-center gap-1">
            <Globe size={13} />
            <span>{t('common.language', 'Lang')}:</span>
          </span>
          <div className="flex items-center bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] p-0.5">
            {languages.map((l) => (
              <button
                key={l.code}
                type="button"
                onClick={() => setLang(l.code)}
                className={`px-2 py-0.5 text-[10px] font-semibold uppercase ${
                  lang === l.code
                    ? 'bg-[var(--cds-interactive)] text-white'
                    : 'text-[var(--cds-text-helper)] hover:text-white'
                }`}
                title={l.name}
              >
                {l.label}
              </button>
            ))}
          </div>
        </div>

        <button
          type="button"
          onClick={onOpenGuide}
          className="w-full h-8 bg-[var(--cds-layer-01)] hover:bg-[#333] border border-[var(--cds-border-subtle)] text-[var(--cds-text-secondary)] hover:text-white text-[11px] font-mono flex items-center justify-center gap-1.5"
        >
          <Help size={14} className="text-[var(--cds-interactive)]" />
          <span>{t('common.guide_tutorial', 'Guide & Tutorial')}</span>
        </button>
      </div>
      </aside>
    </>
  );
}
