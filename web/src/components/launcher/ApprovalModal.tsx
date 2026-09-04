import React from 'react';
import { Security, CheckmarkFilled } from '@carbon/icons-react';
import { useTranslation } from '../../context/I18nContext';

export interface ApprovalModalProps {
  awaitingApproval: { id: string } | null;
  approverName: string;
  setApproverName: (val: string) => void;
  approving: boolean;
  onApprove: () => void;
}

export default function ApprovalModal({
  awaitingApproval,
  approverName,
  setApproverName,
  approving,
  onApprove,
}: ApprovalModalProps) {
  const { t } = useTranslation();

  if (!awaitingApproval) return null;

  return (
    <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-support-warning)] p-4 space-y-3">
      <div className="flex items-center gap-2 text-[var(--cds-support-warning)] font-semibold uppercase text-xs">
        <Security size={16} />
        <span>{t('launcher.guarded_prod_title', 'Guarded Production // Awaiting Approval')}</span>
      </div>
      <p className="text-[var(--cds-text-secondary)] text-[11px]">
        {t('launcher.guarded_prod_desc', 'Run %s targets production and requires approval.', [awaitingApproval.id])}
      </p>
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder={t('launcher.approver_placeholder', 'Approver name/handle')}
          value={approverName}
          onChange={e => setApproverName(e.target.value)}
          className="flex-1 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-2 text-white text-xs"
        />
        <button
          type="button"
          onClick={onApprove}
          disabled={approving || !approverName.trim()}
          className="px-3 py-2 bg-[var(--cds-support-warning)] hover:bg-[var(--cds-support-warning-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-black font-semibold flex items-center gap-1.5"
        >
          <CheckmarkFilled size={14} />
          <span>{approving ? t('launcher.approving', 'Approving...') : t('launcher.approve_and_watch', 'Approve & Watch')}</span>
        </button>
      </div>
    </div>
  );
}
