import React, { ReactNode } from 'react';

export interface SectionTileProps {
  title?: string;
  eyebrow?: string;
  hint?: string;
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
}

export default function SectionTile({
  title,
  eyebrow,
  hint,
  actions,
  children,
  className = '',
}: SectionTileProps) {
  return (
    <div className={`cds-tile p-5 space-y-4 ${className}`}>
      {(title || eyebrow || actions) && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--cds-border-subtle)] pb-3">
          <div>
            {eyebrow && <div className="cds-tile-eyebrow">{eyebrow}</div>}
            {title && (
              <h3 className="text-sm font-semibold text-[var(--cds-text-primary)] font-mono flex items-center gap-2">
                <span>{title}</span>
                {hint && <span className="text-xs text-[var(--cds-text-helper)] font-normal">({hint})</span>}
              </h3>
            )}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      {children}
    </div>
  );
}
