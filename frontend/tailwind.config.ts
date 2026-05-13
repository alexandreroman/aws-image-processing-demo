import type { Config } from 'tailwindcss';

export default {
  content: ['./app/**/*.{vue,ts,js}'],
  theme: {
    extend: {
      colors: {
        // Temporal-ish palette.
        primary: {
          DEFAULT: '#127FBF',
          50: '#E8F4FB',
          100: '#CDE6F4',
          200: '#9ACDE9',
          300: '#67B4DE',
          400: '#349BD2',
          500: '#127FBF',
          600: '#0F6BA1',
          700: '#0B5784',
          800: '#084266',
          900: '#052E48',
        },
        accent: {
          DEFAULT: '#FF8200',
          50: '#FFF1E0',
          100: '#FFE0BD',
          200: '#FFC178',
          300: '#FFA233',
          400: '#FF8200',
          500: '#E07300',
          600: '#B85F00',
        },
        ink: {
          50: '#F8FAFC',
          100: '#F1F5F9',
          200: '#E2E8F0',
          300: '#CBD5E1',
          400: '#94A3B8',
          500: '#64748B',
          600: '#475569',
          700: '#334155',
          800: '#1E293B',
          900: '#0F172A',
        },
      },
      fontFamily: {
        sans: [
          'Inter',
          'ui-sans-serif',
          'system-ui',
          '-apple-system',
          'Segoe UI',
          'Roboto',
          'sans-serif',
        ],
        mono: [
          'JetBrains Mono',
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Consolas',
          'monospace',
        ],
      },
      boxShadow: {
        card: '0 1px 2px 0 rgb(15 23 42 / 0.04), 0 1px 3px 0 rgb(15 23 42 / 0.06)',
      },
      animation: {
        'fade-in': 'fadeIn 220ms ease-out both',
        'slide-in-right': 'slideInRight 220ms ease-out both',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', transform: 'translateY(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(16px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
