/**
 * Utility functions for standardizing time and latency unit conversions across the application.
 */

export const nsToSec = (ns: number | null | undefined): number => {
  return (ns || 0) / 1e9;
};

export const secToNs = (sec: number | null | undefined): number => {
  return Math.round((sec || 0) * 1e9);
};

export const nsToMs = (ns: number | null | undefined): number => {
  return (ns || 0) / 1e6;
};

export const msToNs = (ms: number | null | undefined): number => {
  return Math.round((ms || 0) * 1e6);
};

export const formatMs = (ns: number | null | undefined, decimals = 1): string => {
  if (ns === undefined || ns === null) return '0.0 ms';
  return `${(ns / 1e6).toFixed(decimals)} ms`;
};

export const formatPercent = (fraction: number | null | undefined, decimals = 1): string => {
  if (fraction === undefined || fraction === null) return '0.0%';
  return `${(fraction * 100).toFixed(decimals)}%`;
};
