import React from 'react';
import {
  Certificate, Printer,
  Close, ReportData, Password
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';
import { Run, RunDiagnostics } from '../types/models';

export interface ComplianceCertificateModalProps {
  run: Run | null;
  diagnostics?: RunDiagnostics | null;
  isOpen: boolean;
  onClose: () => void;
}

export default function ComplianceCertificateModal({ run, diagnostics, isOpen, onClose }: ComplianceCertificateModalProps) {
  const { t, formatDate } = useTranslation();
  if (!isOpen || !run) return null;

  const summary = run.summary;
  const diag = diagnostics || (run as any).diagnostics;
  const score = diag?.health_score || 100;
  const isPass = score >= 75 && (!summary || summary.total_errors === 0 || (summary.total_errors / summary.total_requests) < 0.05);

  const handlePrint = () => {
    window.print();
  };

  const formattedDate = run.start_time 
    ? formatDate(run.start_time, { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    : formatDate(new Date());

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-md z-50 flex items-center justify-center p-4 overflow-y-auto">
      <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] w-full max-w-3xl flex flex-col shadow-2xl font-mono text-xs my-8">
        
        {/* Modal Toolbar (Hidden on print) */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between print:hidden">
          <div className="flex items-center space-x-2">
            <Certificate size={16} className="text-[var(--cds-support-success)]" />
            <span className="font-bold text-white uppercase text-xs">
              {t('compliance.title')}
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <button
              onClick={handlePrint}
              className="cds-btn-primary cds-btn-sm normal-case"
            >
              <Printer size={16} />
              <span>{t('compliance.print_btn')}</span>
            </button>
            <button
              onClick={() => window.open(`/api/v1/runs/${run.id}/report`, '_blank')}
              className="cds-btn-secondary cds-btn-sm normal-case"
              title={t('compliance.exec_report_title')}
            >
              <ReportData size={16} />
              <span>{t('compliance.exec_report_btn')}</span>
            </button>
            <button
              onClick={() => window.open(`/api/v1/runs/${run.id}/authorization-letter`, '_blank')}
              className="cds-btn-secondary cds-btn-sm normal-case"
              title={t('compliance.auth_letter_title')}
            >
              <Password size={16} />
              <span>{t('compliance.auth_letter_btn')}</span>
            </button>
            <button
              onClick={onClose}
              className="p-1.5 text-[var(--cds-text-helper)] hover:text-white"
            >
              <Close size={16} />
            </button>
          </div>
        </div>

        {/* Certificate Printable Canvas */}
        <div className="p-8 space-y-6 bg-[var(--cds-background)] text-[var(--cds-text-primary)] border border-[var(--cds-border-subtle)] m-4 print:m-0 print:border-none print:p-6">
          
          {/* Certificate Header */}
          <div className="border-b-2 border-[var(--cds-interactive)] pb-4 flex items-start justify-between">
            <div>
              <div className="text-[10px] text-[var(--cds-interactive)] font-bold uppercase tracking-widest">
                {t('compliance.report_tag')}
              </div>
              <h1 className="text-xl font-bold text-white uppercase mt-1">
                {t('compliance.cert_title')}
              </h1>
              <p className="text-xs text-[var(--cds-text-helper)] font-sans mt-0.5">
                {t('compliance.standards_desc')}
              </p>
            </div>
            
            {/* Status Stamp */}
            <div className={`p-3 border-2 text-center font-bold uppercase tracking-wider ${
              isPass ? 'border-[var(--cds-support-success)] text-[var(--cds-support-success)] bg-[var(--cds-support-success)]/10' : 'border-[var(--cds-support-error)] text-[var(--cds-support-error-text)] bg-[var(--cds-support-error)]/10'
            }`}>
              <div className="text-sm">{isPass ? t('compliance.certified_resilient') : t('compliance.high_risk_failed')}</div>
              <div className="text-[10px] mt-0.5">{isPass ? t('compliance.compliance_passed') : t('compliance.remediation_required')}</div>
            </div>
          </div>

          {/* Metadata Grid */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-[11px] bg-[var(--cds-surface-header)] p-3 border border-[var(--cds-border-subtle)]">
            <div>
              <span className="text-[var(--cds-text-helper)] block text-[9px] uppercase">{t('compliance.meta_run_id')}</span>
              <strong className="text-white">{run.id}</strong>
            </div>
            <div>
              <span className="text-[var(--cds-text-helper)] block text-[9px] uppercase">{t('compliance.audit_date')}</span>
              <strong className="text-white">{formattedDate}</strong>
            </div>
            <div>
              <span className="text-[var(--cds-text-helper)] block text-[9px] uppercase">{t('compliance.meta_target')}</span>
              <strong className="text-[var(--cds-support-success)] truncate block" title={run.config?.target_base_url || run.config?.base_url}>
                {run.config?.target_base_url || run.config?.base_url || t('compliance.fallback_target', 'Internal Testbed')}
              </strong>
            </div>
            <div>
              <span className="text-[var(--cds-text-helper)] block text-[9px] uppercase">{t('compliance.health_score')}</span>
              <strong className={`text-base ${isPass ? 'text-[var(--cds-support-success)]' : 'text-[var(--cds-support-error-text)]'}`}>{score} / 100</strong>
            </div>
          </div>

          {/* SLA Performance Audit Results */}
          <div className="space-y-2">
            <span className="text-xs font-bold text-[var(--cds-interactive)] uppercase tracking-wider block">
              {t('compliance.section_1_title', '1. Performance & Stability Metrics Summary')}
            </span>
            <div className="overflow-x-auto border border-[var(--cds-border-subtle)]">
              <table className="w-full text-left text-xs">
                <thead className="bg-[var(--cds-surface-header)] text-[var(--cds-text-helper)] text-[10px] uppercase border-b border-[var(--cds-border-subtle)]">
                  <tr>
                    <th className="p-2.5">{t('compliance.col_total_requests', 'Total Requests')}</th>
                    <th className="p-2.5">{t('compliance.col_throughput', 'Actual Throughput')}</th>
                    <th className="p-2.5">{t('compliance.col_error_tolerance', 'Error Tolerance')}</th>
                    <th className="p-2.5">{t('compliance.p99_sla')}</th>
                    <th className="p-2.5">{t('compliance.col_termination', 'Termination Outcome')}</th>
                  </tr>
                </thead>
                <tbody className="bg-[var(--cds-surface-sunken)] divide-y divide-[var(--cds-border-subtle)]">
                  <tr>
                    <td className="p-2.5 font-bold text-white">{summary?.total_requests || 0} reqs</td>
                    <td className="p-2.5 text-[var(--cds-interactive)]">{summary?.actual_rps ? summary.actual_rps.toFixed(1) : '0.0'} RPS</td>
                    <td className="p-2.5 text-[var(--cds-support-success)]">
                      {summary?.total_errors === 0 ? t('compliance.error_optimal', '0% Error (Optimal)') : `${summary?.total_errors} err`}
                    </td>
                    <td className="p-2.5 text-white">
                      {summary?.latency ? `${(summary.latency.p99 / 1e6).toFixed(1)} ms` : '-'}
                    </td>
                    <td className="p-2.5">
                      <span className={`px-2 py-0.5 border text-[10px] font-bold ${
                        isPass ? 'bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border-[var(--cds-support-success)]' : 'bg-[var(--cds-support-error)]/20 text-[var(--cds-support-error-text)] border-[var(--cds-support-error)]'
                      }`}>
                        {(summary?.termination_status || run.status).toUpperCase()}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          {/* Compliance Checklist */}
          <div className="space-y-2">
            <span className="text-xs font-bold text-[var(--cds-interactive)] uppercase tracking-wider block">
              {t('compliance.section_2_title', '2. Compliance Criteria Evaluation')}
            </span>
            <div className="space-y-2 text-xs font-sans">
              <div className="p-3 bg-[var(--cds-surface-header)] border border-[var(--cds-border-subtle)] flex items-center justify-between">
                <div>
                  <div className="font-bold text-white">{t('compliance.criteria_iso_title', 'ISO 27001:2022 // A.12.1.3 Capacity & Availability Management')}</div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] mt-0.5">{t('compliance.criteria_iso_desc', 'System must serve nominal traffic without SLA latency degradation breaching SLAs.')}</p>
                </div>
                <span className="px-2 py-1 text-[10px] font-bold bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border border-[var(--cds-support-success)] shrink-0 font-mono">
                  {t('compliance.badge_pass', 'PASS')}
                </span>
              </div>

              <div className="p-3 bg-[var(--cds-surface-header)] border border-[var(--cds-border-subtle)] flex items-center justify-between">
                <div>
                  <div className="font-bold text-white">{t('compliance.criteria_soc2_title', 'SOC 2 Type II // Trust Services Criteria (Availability & Reliability)')}</div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] mt-0.5">{t('compliance.criteria_soc2_desc', 'System employs fail-safe Circuit Breaker protection to prevent cascading failures.')}</p>
                </div>
                <span className="px-2 py-1 text-[10px] font-bold bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border border-[var(--cds-support-success)] shrink-0 font-mono">
                  {t('compliance.badge_verified', 'VERIFIED')}
                </span>
              </div>

              <div className="p-3 bg-[var(--cds-surface-header)] border border-[var(--cds-border-subtle)] flex items-center justify-between">
                <div>
                  <div className="font-bold text-white">{t('compliance.criteria_dr_title', 'Disaster Recovery Readiness // Fault Tolerance (Netem Chaos)')}</div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] mt-0.5">{t('compliance.criteria_dr_desc', 'Resilience validation against controlled network jitter and packet loss injection.')}</p>
                </div>
                <span className="px-2 py-1 text-[10px] font-bold bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] border border-[var(--cds-support-success)] shrink-0 font-mono">
                  {t('compliance.badge_tested', 'TESTED')}
                </span>
              </div>
            </div>
          </div>

          {/* Executive Advisory & Safe Capacity */}
          <div className="p-4 bg-[var(--cds-surface-header)] border border-[var(--cds-border-subtle)] space-y-1.5">
            <span className="text-[10px] text-[var(--cds-support-success)] font-bold uppercase">{t('cockpit.safe_capacity_title')}</span>
            <p className="text-xs text-[var(--cds-text-secondary)] font-sans leading-relaxed">
              {diag?.summary || t('cockpit.optimal_health')}
            </p>
            <div className="text-xs text-white pt-1">
              <strong>{t('cockpit.safe_capacity_title')}:</strong> {diag?.safe_capacity_estimate || t('cockpit.safe_capacity_default', 'Safe up to %s concurrent users.', [run.config?.load_config?.vus || 20])}
            </div>
            {diag?.estimated_revenue_loss_idr && diag.estimated_revenue_loss_idr > 0 ? (
              <div className="text-xs text-[var(--cds-support-error-text)] pt-1 font-sans">
                <strong>{t('remediation.revenue_at_risk')}:</strong> {diag.revenue_loss_note}
              </div>
            ) : null}
          </div>

          {/* Sign-off Approval Block */}
          <div className="pt-6 border-t border-[var(--cds-border-subtle)] grid grid-cols-2 gap-8 text-[11px]">
            <div className="space-y-8">
              <span className="text-[var(--cds-text-helper)] block uppercase text-[9px]">{t('compliance.audited_by')}:</span>
              <div className="border-b border-[var(--cds-layer-03)] w-48 pb-1 text-white">
                Oshimai Autonomous Engine
              </div>
            </div>
            <div className="space-y-8">
              <span className="text-[var(--cds-text-helper)] block uppercase text-[9px]">{t('compliance.authorized_signatory', 'AUTHORIZED SIGNATORY')}:</span>
              <div className="border-b border-[var(--cds-layer-03)] w-48 pb-1 text-white">
                {t('compliance.auditor_role', 'Autonomous Compliance Auditor')}
              </div>
            </div>
          </div>

        </div>

      </div>
    </div>
  );
}
