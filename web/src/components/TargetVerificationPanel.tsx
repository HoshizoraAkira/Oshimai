import React, { useState, useEffect, useCallback } from 'react';
import { Security, CheckmarkFilled, WarningAltFilled, Locked, Unlocked } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { verificationService, ChallengeResponse } from '../services/verificationService';
import { TargetVerifyStatus, ToastType } from '../types/models';

export interface TargetVerificationPanelProps {
  targetBaseUrl: string;
  showToast: (msg: string, type?: ToastType) => void;
  onStatusChange?: (status: TargetVerifyStatus | null) => void;
}

export default function TargetVerificationPanel({ targetBaseUrl, showToast, onStatusChange }: TargetVerificationPanelProps) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<TargetVerifyStatus | null>(null);
  const [challenge, setChallenge] = useState<ChallengeResponse | null>(null);
  const [checking, setChecking] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const fetchStatus = useCallback(async () => {
    if (!targetBaseUrl || !targetBaseUrl.trim()) {
      setStatus(null);
      onStatusChange?.(null);
      return;
    }
    setChecking(true);
    try {
      const data = await verificationService.getVerifyStatus(targetBaseUrl);
      setStatus(data);
      onStatusChange?.(data);
    } catch (err: any) {
      // A 400 here just means the in-progress URL isn't parseable yet (expected while typing,
      // debounced every keystroke) — not worth a toast. A network failure (status 0) or a 5xx is
      // a real problem: status===null renders nothing below, which would otherwise make the
      // public-target warning banner silently vanish exactly when it should flag an unverified
      // target, so that case gets surfaced instead of swallowed.
      if (!err?.status || err.status >= 500) {
        showToast(t('toasts.verify_status_check_failed', 'Could not check target verification status: %s', [err?.message || 'unknown error']), 'error');
      }
      setStatus(null);
      onStatusChange?.(null);
    } finally {
      setChecking(false);
    }
  }, [targetBaseUrl, onStatusChange, showToast, t]);

  useEffect(() => {
    const tId = setTimeout(fetchStatus, 400); // Debounce while typing
    return () => clearTimeout(tId);
  }, [fetchStatus]);

  const requestChallenge = async () => {
    try {
      const data = await verificationService.requestChallenge(targetBaseUrl);
      setChallenge(data);
    } catch (e: any) {
      showToast(t('toasts.challenge_failed', 'Failed to request challenge: %s', [e.message]), 'error');
    }
  };

  const confirmChallenge = async () => {
    setConfirming(true);
    try {
      const data = await verificationService.confirmChallenge(targetBaseUrl);
      showToast(t('toasts.target_verified', 'Target verified via %s!', [data.method]), 'success');
      setChallenge(null);
      fetchStatus();
    } catch (e: any) {
      showToast(t('toasts.confirm_failed', 'Failed to confirm: %s', [e.message]), 'error');
    } finally {
      setConfirming(false);
    }
  };

  if (!status) return null;

  if (status.is_private) {
    return (
      <div className="flex items-center gap-2 text-[11px] text-[var(--cds-text-helper)] px-1">
        <Unlocked size={14} />
        <span>{t('verification.private_target', 'Internal/local target (%s) — ownership verification not required.', [status.host])}</span>
      </div>
    );
  }

  if (status.is_blocked) {
    return (
      <div className="cds-notification-error text-[11px] text-[var(--cds-support-error-text)] items-center">
        <WarningAltFilled size={14} className="shrink-0" />
        <span>{t('verification.blocked_target', '%s is a well-known public domain and is permanently blocked from being tested.', [status.host])}</span>
      </div>
    );
  }

  if (status.is_verified) {
    return (
      <div className="flex items-center gap-2 text-[11px] text-[var(--cds-support-success-text)] px-1">
        <CheckmarkFilled size={14} />
        <span>{t('verification.verified_target', '%s is verified as belonging to you. Safe to target.', [status.host])}</span>
      </div>
    );
  }

  return (
    <div className="cds-notification-warning flex-col items-stretch gap-3">
      <div className="flex items-center gap-2 text-[11px] text-[var(--cds-support-warning)] font-semibold">
        <Locked size={14} />
        <span>{t('verification.unverified_target', '%s is a public target and ownership has not yet been verified.', [status.host])}</span>
      </div>

      {!challenge ? (
        <button
          type="button"
          onClick={requestChallenge}
          disabled={checking}
          className="cds-btn-secondary cds-btn-sm normal-case self-start"
        >
          <Security size={14} />
          <span>{t('verification.request_challenge_btn')}</span>
        </button>
      ) : (
        <div className="space-y-2 text-[11px] font-mono">
          <p className="text-[var(--cds-text-secondary)]">{t('verification.instructions_title')}</p>
          <div className="p-2.5 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)]">
            <div className="text-[var(--cds-text-helper)]">{t('verification.dns_record_label')}</div>
            <div className="text-[var(--cds-link)] break-all">{challenge.dns_record_name}</div>
            <div className="text-[var(--cds-link)] break-all">{challenge.dns_record_value}</div>
          </div>
          <div className="p-2.5 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)]">
            <div className="text-[var(--cds-text-helper)]">{t('verification.or_file_label')}</div>
            <div className="text-[var(--cds-link)] break-all">{status.host}{challenge.well_known_path}</div>
            <div className="text-[var(--cds-text-helper)]">{t('verification.exact_content_label')}</div>
            <div className="text-[var(--cds-link)] break-all">{challenge.well_known_content}</div>
          </div>
          <button
            type="button"
            onClick={confirmChallenge}
            disabled={confirming}
            className="cds-btn-primary cds-btn-sm normal-case"
          >
            <CheckmarkFilled size={14} />
            <span>{confirming ? t('verification.btn_checking') : t('verification.btn_confirm')}</span>
          </button>
        </div>
      )}
    </div>
  );
}
