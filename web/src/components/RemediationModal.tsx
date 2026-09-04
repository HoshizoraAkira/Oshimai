import React, { useEffect, useState } from 'react';
import { Close, Copy, Checkmark, Terminal, VolumeUp, VolumeMute } from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { runsService } from '../services/runsService';
import { RunDiagnostics, ToastType } from '../types/models';

export interface RemediationModalProps {
  runId: string | null;
  isOpen: boolean;
  onClose: () => void;
  showToast: (msg: string, type?: ToastType) => void;
}

export default function RemediationModal({ runId, isOpen, onClose, showToast }: RemediationModalProps) {
  const { t, lang, localeCode } = useTranslation();
  const [diag, setDiag] = useState<RunDiagnostics | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);
  const [speaking, setSpeaking] = useState(false);

  useEffect(() => {
    if (!runId || !isOpen) return;
    setLoading(true);
    setCopied(false);

    runsService.getRunDiagnostics(runId)
      .then(data => {
        setDiag(data);
        setLoading(false);
      })
      .catch(err => {
        console.error('Diagnostics fetch error:', err);
        setLoading(false);
      });
  }, [runId, isOpen]);

  // Cleanup: stop any in-flight speech synthesis on unmount. Declared before the `isOpen` early
  // return below so every hook in this component always runs on every render — conditionally
  // skipping a hook (this one used to sit after the early return) violates the Rules of Hooks and
  // crashes the whole app with a hook-count-mismatch error the moment isOpen toggles.
  useEffect(() => () => window.speechSynthesis?.cancel(), []);

  if (!isOpen) return null;

  // Audio summary: reads the diagnosis aloud using the browser's built-in
  // Web Speech API in the current active locale (id-ID, en-US, ja-JP).
  const handleToggleSpeech = () => {
    if (!('speechSynthesis' in window)) {
      showToast(t('toasts.tts_not_supported', 'This browser does not support text-to-speech.'), 'error');
      return;
    }
    if (speaking) {
      window.speechSynthesis.cancel();
      setSpeaking(false);
      return;
    }
    const text = [diag?.status_label, diag?.summary, diag?.revenue_loss_note]
      .filter(Boolean).join('. ');
    if (!text) return;

    const utterance = new SpeechSynthesisUtterance(text);
    utterance.lang = localeCode || 'en-US';
    const langPrefix = (lang === 'jp' || lang === 'ja') ? 'ja' : lang === 'id' ? 'id' : 'en';
    const voice = window.speechSynthesis.getVoices().find(v => v.lang?.toLowerCase().startsWith(langPrefix));
    if (voice) utterance.voice = voice;
    utterance.onend = () => setSpeaking(false);
    utterance.onerror = () => setSpeaking(false);

    window.speechSynthesis.cancel();
    window.speechSynthesis.speak(utterance);
    setSpeaking(true);
  };

  const handleCopyPatch = () => {
    if (!diag?.suggested_config_patch) return;
    navigator.clipboard.writeText(diag.suggested_config_patch).then(() => {
      setCopied(true);
      showToast(t('toasts.patch_copied', 'Config patch copied to clipboard!'), 'success');
      setTimeout(() => setCopied(false), 2500);
    });
  };

  const score = diag?.health_score || 100;
  const scoreBadge = score >= 85 
    ? 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]' 
    : score >= 60 
    ? 'bg-[var(--cds-support-warning)]/20 text-[var(--cds-support-warning)] border-[var(--cds-support-warning)]' 
    : 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]';

  return (
    <div className="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] w-full max-w-3xl max-h-[90vh] flex flex-col shadow-2xl font-mono">
        
        {/* Modal Header */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <span className="text-xs text-[var(--cds-interactive)] font-bold uppercase">{t('remediation.title')}</span>
            <span className={`px-2 py-0.5 border text-xs font-bold ${scoreBadge}`}>
              {t('remediation.health_badge', 'HEALTH: %s/100', [score])}
            </span>
            {diag && (
              <button
                onClick={handleToggleSpeech}
                className="cds-btn-ghost cds-btn-sm normal-case border border-[var(--cds-border-subtle)]"
                title={t('remediation.listen_title')}
              >
                {speaking ? <VolumeMute size={14} /> : <VolumeUp size={14} />}
                <span>{speaking ? t('remediation.stop_listening') : t('remediation.listen_summary')}</span>
              </button>
            )}
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-[var(--cds-text-helper)] hover:text-white hover:bg-[var(--cds-border-subtle)] transition-colors"
          >
            <Close size={16} />
          </button>
        </div>

        {/* Modal Body */}
        <div className="flex-1 overflow-y-auto p-5 space-y-4 text-xs">
          {loading ? (
            <div className="p-8 text-center text-[var(--cds-text-helper)]">Evaluating performance traces and bottlenecks...</div>
          ) : diag ? (
            <>
              {/* Executive Summary */}
              <div className="p-3.5 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1.5">
                <span className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold">{t('remediation.business_impact')}</span>
                <p className="text-sm text-white font-sans leading-relaxed">
                  {diag.summary || t('cockpit.optimal_health')}
                </p>
                <div className="text-[11px] text-[var(--cds-support-success-text)]">
                  {diag.status_label}
                </div>
                {diag.estimated_revenue_loss_idr && diag.estimated_revenue_loss_idr > 0 ? (
                  <div className="text-[11px] text-[var(--cds-support-error-text)] font-sans pt-1 border-t border-[var(--cds-border-subtle)] mt-1.5">
                    {diag.revenue_loss_note}
                  </div>
                ) : null}
              </div>

              {/* Root Cause Analysis */}
              <div className="p-3.5 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1">
                <span className="text-[10px] text-[var(--cds-support-error)] uppercase font-bold">{t('remediation.root_cause_analysis')}</span>
                <p className="text-xs text-[var(--cds-text-secondary)] font-mono leading-relaxed">
                  {diag.root_cause || 'No dominant bottleneck detected across CPU, memory, or database.'}
                </p>
              </div>

              {/* Detected Bottlenecks */}
              <div className="space-y-2">
                <span className="text-[11px] text-[var(--cds-support-warning)] uppercase font-bold">{t('remediation.primary_chokepoints')}</span>
                {diag.detected_issues && diag.detected_issues.length > 0 ? (
                  diag.detected_issues.map((iss, iIdx) => (
                    <div key={iIdx} className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-white">{iss.title}</span>
                        <span className="px-1.5 py-0.5 text-[10px] border border-[var(--cds-support-error)] text-[var(--cds-support-error-text)]">
                          {iss.severity}
                        </span>
                      </div>
                      <p className="text-xs text-[var(--cds-text-helper)] font-sans">{iss.description}</p>
                    </div>
                  ))
                ) : (
                  <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-support-success)] text-[var(--cds-support-success-text)]">
                    Zero critical bottlenecks or 5xx failures observed during test execution.
                  </div>
                )}
              </div>

              {/* Actionable Engineering Fixes */}
              <div className="space-y-2">
                <span className="text-[11px] text-[var(--cds-support-success)] uppercase font-bold">{t('drawer.actions_title')}</span>
                <ol className="list-decimal pl-5 space-y-1.5 text-xs text-[var(--cds-text-secondary)] font-sans leading-relaxed">
                  {diag.actionable_fixes && diag.actionable_fixes.length > 0 ? (
                    diag.actionable_fixes.map((fix, fIdx) => (
                      <li key={fIdx}>{fix}</li>
                    ))
                  ) : (
                    <li>Maintain current architecture and schedule recurring regression tests.</li>
                  )}
                </ol>
              </div>

              {/* Configuration Patch Snippet */}
              {diag.suggested_config_patch && (
                <div className="space-y-2 pt-2 border-t border-[var(--cds-border-subtle)]">
                  <div className="flex items-center justify-between">
                    <span className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold flex items-center gap-1.5">
                      <Terminal size={14} />
                      <span>{t('remediation.suggested_patch')}</span>
                    </span>
                    <button 
                      onClick={handleCopyPatch}
                      className="px-2.5 py-1 bg-[var(--cds-border-subtle)] hover:bg-[var(--cds-layer-03)] text-white text-[11px] flex items-center gap-1 transition-colors"
                    >
                      {copied ? <Checkmark size={14} className="text-[var(--cds-support-success)]" /> : <Copy size={14} />}
                      <span>{copied ? t('remediation.copied') : t('remediation.copy_patch')}</span>
                    </button>
                  </div>
                  <pre className="p-3 bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] text-[var(--cds-support-success)] font-mono text-[11px] overflow-x-auto">
                    {diag.suggested_config_patch}
                  </pre>
                </div>
              )}
            </>
          ) : (
            <div className="p-8 text-center text-[var(--cds-text-helper)]">No diagnostic report available for this run.</div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-t border-[var(--cds-border-subtle)] flex justify-end">
          <button 
            onClick={onClose}
            className="px-4 py-1.5 bg-[var(--cds-border-subtle)] hover:bg-[var(--cds-layer-03)] text-white text-xs font-semibold"
          >
            {t('common.close')}
          </button>
        </div>

      </div>
    </div>
  );
}
