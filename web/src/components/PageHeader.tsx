import React, { ReactNode } from 'react';

export interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}

export default function PageHeader({ eyebrow, title, description, actions }: PageHeaderProps) {
  return (
    <div className="cds-page-header">
      <div>
        {eyebrow && <div className="cds-page-eyebrow">{eyebrow}</div>}
        <h1 className="cds-page-title">{title}</h1>
        {description && <p className="cds-page-desc">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );
}
