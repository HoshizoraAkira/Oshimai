/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        carbon: {
          bg: '#161616',
          layer1: '#262626',
          layer2: '#393939',
          layer3: '#525252',
          border: '#393939',
          text1: '#f4f4f4',
          text2: '#c6c6c6',
          helper: '#8d8d8d',
          blue: '#0f62fe',
          green: '#24a148',
          red: '#da1e28',
          yellow: '#f1c21b',
          purple: '#8a3ffc',
        }
      },
      fontFamily: {
        mono: ['IBM Plex Mono', 'monospace'],
        sans: ['IBM Plex Sans', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
