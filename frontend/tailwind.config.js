/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"DM Sans"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      colors: {
        cream: '#faf9f5',
        canvas: '#ffffff',
        warmgray: {
          50: '#f7f6f1',
          100: '#f0eee6',
          200: '#e5e2d6',
          300: '#cfcbbe',
          400: '#a09c8e',
          500: '#73706a',
          600: '#5a5853',
          700: '#3f3e3a',
          800: '#262522',
          900: '#1f1e1c',
        },
        coral: {
          50: '#fdf4ee',
          100: '#fae6d8',
          200: '#f4c9a8',
          300: '#eda575',
          400: '#e0814d',
          500: '#c96442',
          600: '#b34c2f',
          700: '#933a25',
          800: '#762f21',
        },
        moss: '#506b3a',
      },
      boxShadow: {
        card: '0 1px 2px rgba(31, 30, 28, 0.04), 0 4px 16px -8px rgba(31, 30, 28, 0.08)',
        soft: '0 1px 0 rgba(31, 30, 28, 0.04)',
      },
      letterSpacing: {
        tightish: '-0.015em',
      },
      animation: {
        'pulse-dot': 'pulseDot 1.8s ease-in-out infinite',
      },
      keyframes: {
        pulseDot: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.35' },
        },
      },
    },
  },
  plugins: [],
}
