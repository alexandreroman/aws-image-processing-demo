import type { Config } from 'tailwindcss';

export default {
  content: ['./app/**/*.{vue,ts,js}'],
  theme: {
    extend: {
      colors: {
        // Brand: Temporal cyan-blue — bright variant chosen for legibility on dark.
        primary: {
          DEFAULT: '#2EB5F5',
          50: '#E6F6FE',
          100: '#C2E9FC',
          200: '#8FD6F9',
          300: '#5CC3F7',
          400: '#2EB5F5',
          500: '#0EA5E9',
          600: '#0284C7',
          700: '#0369A1',
          800: '#075985',
          900: '#0C4A6E',
        },
        // AWS orange — secondary accent.
        accent: {
          DEFAULT: '#FF9900',
          50: '#FFF3E0',
          100: '#FFE0B2',
          200: '#FFCC80',
          300: '#FFB74D',
          400: '#FFA726',
          500: '#FF9900',
          600: '#FB8C00',
          700: '#F57C00',
        },
        // Temporal-leaning purple for secondary highlights (running state, etc.).
        iris: {
          DEFAULT: '#A78BFA',
          50: '#F5F3FF',
          100: '#EDE9FE',
          200: '#DDD6FE',
          300: '#C4B5FD',
          400: '#A78BFA',
          500: '#8B5CF6',
          600: '#7C3AED',
          700: '#6D28D9',
        },
        // Dark surfaces. `bg` is the page; `surface*` are card layers.
        bg: '#0A0E1A',
        surface: {
          DEFAULT: '#111827',
          elevated: '#1A2236',
          border: '#1F2A40',
          hover: '#1E2942',
        },
        // Cool gray ramp tuned for dark UI (kept name `ink` for class-name
        // compatibility with existing components).
        ink: {
          50: '#F8FAFC',
          100: '#E5EBF5',
          200: '#CBD5E1',
          300: '#94A3B8',
          400: '#64748B',
          500: '#475569',
          600: '#334155',
          700: '#1E293B',
          800: '#111827',
          900: '#0A0E1A',
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
        card: '0 1px 0 0 rgb(255 255 255 / 0.04) inset, 0 8px 24px -12px rgb(0 0 0 / 0.6)',
        glow: '0 0 0 1px rgb(46 181 245 / 0.4), 0 6px 24px -4px rgb(46 181 245 / 0.35)',
      },
      backgroundImage: {
        grid: 'radial-gradient(circle at 1px 1px, rgb(255 255 255 / 0.06) 1px, transparent 0)',
      },
      animation: {
        'fade-in': 'fadeIn 220ms ease-out both',
        'slide-in-right': 'slideInRight 220ms ease-out both',
        'pulse-glow': 'pulseGlow 1.6s ease-in-out infinite',
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
        pulseGlow: {
          '0%, 100%': { opacity: '0.5' },
          '50%': { opacity: '1' },
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
