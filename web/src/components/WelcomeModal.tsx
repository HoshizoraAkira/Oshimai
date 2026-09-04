import React, { useState } from 'react';
import { 
  WatsonMachineLearning, ArrowRight, 
  CheckmarkFilled, Close, Chip, Certificate, Globe 
} from '@carbon/icons-react';
import { useTranslation } from '../context/I18nContext';

export interface WelcomeModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function WelcomeModal({ isOpen, onClose }: WelcomeModalProps) {
  const { t, lang, setLang, languages } = useTranslation();
  const [step, setStep] = useState(1);
  const [selectedPersona, setSelectedPersona] = useState<'business' | 'sre'>('business');

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-md z-50 flex items-center justify-center p-4">
      <div className="bg-[var(--cds-layer-01)] border border-[var(--cds-border-subtle)] w-full max-w-2xl flex flex-col shadow-2xl font-mono text-xs">
        
        {/* Modal Header */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-b border-[var(--cds-border-subtle)] flex items-center justify-between gap-3">
          <div className="flex items-center space-x-2.5 min-w-0">
            <WatsonMachineLearning size={18} className="text-[var(--cds-interactive)] shrink-0" />
            <span className="font-bold text-sm text-white uppercase tracking-wider truncate">
              {t('welcome.header', 'Welcome to Oshimai // Out-of-Box Guidance')}
            </span>
          </div>
          <div className="flex items-center space-x-3 shrink-0">
            {/* Language Selector */}
            <div className="flex items-center bg-[var(--cds-background)] border border-[var(--cds-border-subtle)] p-0.5">
              <Globe size={13} className="text-[var(--cds-text-helper)] mx-1.5" />
              {languages.map((l) => (
                <button
                  key={l.code}
                  type="button"
                  onClick={() => setLang(l.code)}
                  className={`px-2 py-0.5 text-[10px] font-semibold uppercase transition-colors ${
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
            <button 
              onClick={onClose}
              className="text-[var(--cds-text-helper)] hover:text-white p-1 transition-colors"
              title={t('common.close', 'Close')}
            >
              <Close size={16} />
            </button>
          </div>
        </div>

        {/* Step Progression Bar */}
        <div className="flex border-b border-[var(--cds-border-subtle)] bg-[var(--cds-background)]">
          <div className={`flex-1 py-2 text-center border-b-2 text-[11px] ${step === 1 ? 'border-[var(--cds-interactive)] text-white font-bold' : 'border-transparent text-[var(--cds-text-helper)]'}`}>
            {t('welcome.tab_overview', '1. Overview')}
          </div>
          <div className={`flex-1 py-2 text-center border-b-2 text-[11px] ${step === 2 ? 'border-[var(--cds-interactive)] text-white font-bold' : 'border-transparent text-[var(--cds-text-helper)]'}`}>
            {t('welcome.tab_persona', '2. Choose Persona')}
          </div>
          <div className={`flex-1 py-2 text-center border-b-2 text-[11px] ${step === 3 ? 'border-[var(--cds-interactive)] text-white font-bold' : 'border-transparent text-[var(--cds-text-helper)]'}`}>
            {t('welcome.tab_quickstart', '3. 3-Step Quick Start')}
          </div>
        </div>

        {/* Modal Body */}
        <div className="p-6 space-y-5 flex-1 min-h-[340px]">
          
          {/* STEP 1: Overview */}
          {step === 1 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-base font-bold text-white">
                  {t('welcome.step1_title', 'Digital Twin Resilience & Chaos Platform')}
                </h3>
                <p className="text-[var(--cds-text-secondary)] text-xs font-sans mt-1 leading-relaxed">
                  {t('welcome.step1_desc', 'Oshimai menguji ketahanan sistem modern dengan memadukan simulasi beban pengguna (*load testing*) dan injeksi gangguan jaringan nyata (*network chaos*) untuk mendeteksi bottleneck sebelum terjadi kegagalan di produksi.')}
                </p>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2">
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1">
                  <div className="text-[10px] text-[var(--cds-interactive)] uppercase font-bold">
                    {t('welcome.step1_card1_eyebrow', '01 // Visual Flow')}
                  </div>
                  <div className="font-semibold text-white">
                    {t('welcome.step1_card1_title', 'Draw.io Canvas')}
                  </div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] font-sans">
                    {t('welcome.step1_card1_desc', 'Rancang alur perjalanan user dengan drag-and-drop kartu tanpa koding.')}
                  </p>
                </div>
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1">
                  <div className="text-[10px] text-[var(--cds-support-novel-strong)] uppercase font-bold">
                    {t('welcome.step1_card2_eyebrow', '02 // Real-Time Telemetry')}
                  </div>
                  <div className="font-semibold text-white">
                    {t('welcome.step1_card2_title', '500ms HDR Quantiles')}
                  </div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] font-sans">
                    {t('welcome.step1_card2_desc', 'Pantau latensi P50/P90/P99 mikrodetik dan proteksi circuit breaker.')}
                  </p>
                </div>
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] space-y-1">
                  <div className="text-[10px] text-[var(--cds-support-success)] uppercase font-bold">
                    {t('welcome.step1_card3_eyebrow', '03 // Audit & Compliance')}
                  </div>
                  <div className="font-semibold text-white">
                    {t('welcome.step1_card3_title', 'ISO/SOC2 Reports')}
                  </div>
                  <p className="text-[11px] text-[var(--cds-text-helper)] font-sans">
                    {t('welcome.step1_card3_desc', 'Cetak sertifikat bukti kepatuhan SLA siap audit formal.')}
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* STEP 2: Choose Persona */}
          {step === 2 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-base font-bold text-white">
                  {t('welcome.step2_title', 'Pilih Profil Kebutuhan Anda')}
                </h3>
                <p className="text-[var(--cds-text-secondary)] text-xs font-sans mt-1">
                  {t('welcome.step2_desc', 'Pilih mode penggunaan yang paling sesuai dengan tujuan pengujian Anda hari ini:')}
                </p>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
                <div 
                  onClick={() => setSelectedPersona('business')}
                  className={`p-4 border cursor-pointer transition-all ${
                    selectedPersona === 'business' 
                      ? 'bg-[var(--cds-interactive)]/15 border-[var(--cds-interactive)] ring-1 ring-[var(--cds-interactive)]' 
                      : 'bg-[var(--cds-surface-sunken)] border-[var(--cds-border-subtle)] hover:border-[var(--cds-border-strong)]'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <Certificate size={20} className="text-[var(--cds-support-success)]" />
                    <span className="text-[10px] bg-[var(--cds-support-success)]/20 text-[var(--cds-support-success-text)] px-1.5 py-0.5 border border-[var(--cds-support-success)]">
                      {t('welcome.persona_biz_badge', 'COMPLIANCE & QA')}
                    </span>
                  </div>
                  <h4 className="font-bold text-sm text-white mt-2">
                    {t('welcome.persona_biz_title', 'Vibe Coder / Business Lead')}
                  </h4>
                  <p className="text-[11px] text-[var(--cds-text-secondary)] font-sans mt-1 leading-relaxed">
                    {t('welcome.persona_biz_desc', 'Saya fokus pada validasi flow bisnis, kapasitas aman pengguna, serta butuh Laporan Sertifikasi SLA (ISO 27001 / SOC 2) siap cetak untuk audit & manajemen.')}
                  </p>
                </div>

                <div 
                  onClick={() => setSelectedPersona('sre')}
                  className={`p-4 border cursor-pointer transition-all ${
                    selectedPersona === 'sre' 
                      ? 'bg-[var(--cds-interactive)]/15 border-[var(--cds-interactive)] ring-1 ring-[var(--cds-interactive)]' 
                      : 'bg-[var(--cds-surface-sunken)] border-[var(--cds-border-subtle)] hover:border-[var(--cds-border-strong)]'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <Chip size={20} className="text-[var(--cds-interactive)]" />
                    <span className="text-[10px] bg-[var(--cds-interactive)]/20 text-[var(--cds-link)] px-1.5 py-0.5 border border-[var(--cds-interactive)]">
                      {t('welcome.persona_sre_badge', 'DEVOPS & SRE')}
                    </span>
                  </div>
                  <h4 className="font-bold text-sm text-white mt-2">
                    {t('welcome.persona_sre_title', 'Technical Expert / SRE')}
                  </h4>
                  <p className="text-[11px] text-[var(--cds-text-secondary)] font-sans mt-1 leading-relaxed">
                    {t('welcome.persona_sre_desc', 'Saya butuh metrik presisi tinggi (P99 latency SLA), injeksi kegagalan jaringan (Netem jitter & packet loss), integrasi pipeline CI/CD, dan root cause analysis.')}
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* STEP 3: 3-Step Quick Start Guide */}
          {step === 3 && (
            <div className="space-y-4">
              <div>
                <h3 className="text-base font-bold text-white">
                  {t('welcome.step3_title', '3 Langkah Cepat Memulai Pengujian')}
                </h3>
                <p className="text-[var(--cds-text-secondary)] text-xs font-sans mt-1">
                  {t('welcome.step3_desc', 'Ikuti panduan mudah ini untuk menjalankan pengujian pertama Anda dalam hitungan detik:')}
                </p>
              </div>

              <div className="space-y-3 pt-1">
                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] flex items-start space-x-3">
                  <div className="w-6 h-6 rounded-full bg-[var(--cds-interactive)] text-white flex items-center justify-center font-bold text-xs shrink-0 mt-0.5">1</div>
                  <div>
                    <div className="font-bold text-white">
                      {t('welcome.step3_item1_title', 'Rancang Perjalanan Pengguna di Kanvas')}
                    </div>
                    <p className="text-[11px] text-[var(--cds-text-helper)] font-sans mt-0.5">
                      {t('welcome.step3_item1_desc', 'Buka tab Visual Scenario Builder. Anda bisa drag kartu HTTP, hubungkan panah probabilitas, atau cukup klik template E-Commerce.')}
                    </p>
                  </div>
                </div>

                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] flex items-start space-x-3">
                  <div className="w-6 h-6 rounded-full bg-[var(--cds-interactive)] text-white flex items-center justify-center font-bold text-xs shrink-0 mt-0.5">2</div>
                  <div>
                    <div className="font-bold text-white">
                      {t('welcome.step3_item2_title', 'Tentukan Beban & Parameter Uji')}
                    </div>
                    <p className="text-[11px] text-[var(--cds-text-helper)] font-sans mt-0.5">
                      {t('welcome.step3_item2_desc', 'Klik Execute in Launcher, tentukan jumlah pengunjung bersamaan (VUs), durasi pengujian, dan aktifkan simulasi degradasi jika diinginkan.')}
                    </p>
                  </div>
                </div>

                <div className="p-3 bg-[var(--cds-surface-sunken)] border border-[var(--cds-border-subtle)] flex items-start space-x-3">
                  <div className="w-6 h-6 rounded-full bg-[var(--cds-support-success)] text-white flex items-center justify-center font-bold text-xs shrink-0 mt-0.5">3</div>
                  <div>
                    <div className="font-bold text-white">
                      {t('welcome.step3_item3_title', 'Amati Live Telemetri & Dapatkan Laporan Flow')}
                    </div>
                    <p className="text-[11px] text-[var(--cds-text-helper)] font-sans mt-0.5">
                      {t('welcome.step3_item3_desc', 'Saat pengujian selesai, dashboard langsung menampilkan Flow Heatmap Report, Skor Kesehatan, serta sertifikat kepatuhan audit formal siap cetak.')}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          )}

        </div>

        {/* Modal Footer */}
        <div className="p-4 bg-[var(--cds-surface-header)] border-t border-[var(--cds-border-subtle)] flex items-center justify-between">
          {step > 1 ? (
            <button
              onClick={() => setStep(s => s - 1)}
              className="cds-btn-secondary normal-case"
            >
              {t('common.back', 'Kembali')}
            </button>
          ) : <div></div>}

          {step < 3 ? (
            <button
              onClick={() => setStep(s => s + 1)}
              className="cds-btn-primary normal-case"
            >
              <span>{t('common.next', 'Lanjut')}</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          ) : (
            <button
              onClick={() => {
                localStorage.setItem('oshimai_onboarded', 'true');
                onClose();
              }}
              className="cds-btn-primary normal-case bg-[var(--cds-support-success)] hover:bg-[var(--cds-support-success-hover)]"
            >
              <CheckmarkFilled size={16} />
              <span>{t('welcome.btn_start', 'Mulai Jelajahi Platform')}</span>
            </button>
          )}
        </div>

      </div>
    </div>
  );
}
