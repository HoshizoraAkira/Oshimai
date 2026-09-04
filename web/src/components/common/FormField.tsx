import React, { ReactNode } from 'react';

export interface FormFieldProps {
  label: string;
  helperText?: string;
  helper?: string;
  children: ReactNode;
  className?: string;
}

export default function FormField({ label, helperText, helper, children, className = '' }: FormFieldProps) {
  const text = helperText || helper;
  return (
    <div className={`space-y-1 ${className}`}>
      <label className="cds-label mb-0">{label}</label>
      {children}
      {text && <p className="cds-helper-text mb-0">{text}</p>}
    </div>
  );
}
