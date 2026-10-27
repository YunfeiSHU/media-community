/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: 'var(--brand-pink)',
          hover: 'var(--brand-pink-hover)',
          blue: 'var(--brand-blue)',
        },
        page: 'var(--bg-page)',
        card: 'var(--bg-card)',
        elevated: 'var(--bg-elevated)',
        subtle: 'var(--bg-subtle)',
        ink: {
          DEFAULT: 'var(--text-primary)',
          secondary: 'var(--text-secondary)',
          tertiary: 'var(--text-tertiary)',
        },
        line: 'var(--border)',
        divider: 'var(--divider)',
        info: 'var(--brand-blue)',
        success: 'var(--success)',
        warning: 'var(--warning)',
        danger: 'var(--danger)',
      },
      fontFamily: {
        sans: 'var(--font-sans)',
      },
      fontSize: {
        micro: ['11px', '16px'],
        meta: ['12px', '18px'],
        body: ['14px', '22px'],
        h2: ['16px', '24px'],
        h1: ['20px', '28px'],
        display: ['24px', '32px'],
      },
      borderRadius: {
        xs: 'var(--radius-xs)',
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
      },
      boxShadow: {
        card: 'var(--shadow-card)',
        hover: 'var(--shadow-hover)',
        pop: 'var(--shadow-pop)',
      },
      maxWidth: {
        content: 'var(--content-max)',
      },
      height: {
        header: 'var(--header-h)',
      },
      spacing: {
        18: '4.5rem',
      },
    },
  },
  plugins: [],
};
