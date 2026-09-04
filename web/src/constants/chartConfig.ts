/**
 * Shared Chart.js options styled according to the IBM Carbon Design System dark theme.
 */

export const CARBON_CHART_OPTIONS = {
  responsive: true,
  maintainAspectRatio: false,
  animation: false as const,
  plugins: {
    legend: {
      labels: {
        color: '#c6c6c6',
        font: { family: 'IBM Plex Mono', size: 10 },
        boxWidth: 10,
      },
    },
  },
  scales: {
    x: {
      grid: { color: 'rgba(255, 255, 255, 0.05)' },
      ticks: {
        color: '#8d8d8d',
        maxRotation: 0,
        font: { family: 'IBM Plex Mono', size: 9 },
      },
    },
    y: {
      grid: { color: 'rgba(255, 255, 255, 0.05)' },
      ticks: {
        color: '#8d8d8d',
        font: { family: 'IBM Plex Mono', size: 9 },
      },
    },
  },
};

/**
 * Creates dual-axis chart options by cloning Carbon options and attaching an independent right Y-axis (y1).
 */
export const createDualAxisOptions = (rightAxisTickColor = '#4589ff') => {
  const options = JSON.parse(JSON.stringify(CARBON_CHART_OPTIONS));
  options.scales.y1 = {
    position: 'right',
    grid: { drawOnChartArea: false },
    ticks: {
      color: rightAxisTickColor,
      font: { family: 'IBM Plex Mono', size: 9 },
    },
  };
  return options;
};
