import React from 'react';
import { Close } from '@carbon/icons-react';
import { ToastMessage, ToastType } from '../../types/models';

export interface ToastContainerProps {
  toasts: ToastMessage[];
  onDismiss: (id: number) => void;
}

const NOTIF_CLASS: Record<ToastType, string> = {
  error: 'cds-notification-error',
  success: 'cds-notification-success',
  warning: 'cds-notification-warning',
  info: 'cds-notification-info',
};

export default function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  if (!toasts || toasts.length === 0) return null;

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none font-mono">
      {toasts.map(t => (
        <div
          key={t.id}
          className={`${NOTIF_CLASS[t.type] || NOTIF_CLASS.info} cds-enter pointer-events-auto shadow-xl min-w-[280px] max-w-sm text-xs`}
        >
          <div className="flex-1 leading-relaxed pt-0.5">{t.message}</div>
          <button
            onClick={() => onDismiss(t.id)}
            className="text-[var(--cds-text-helper)] hover:text-white shrink-0"
            aria-label="Dismiss notification"
          >
            <Close size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
