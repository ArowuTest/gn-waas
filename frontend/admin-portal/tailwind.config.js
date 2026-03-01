/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // GN-WAAS Brand Palette
        brand: {
          50:  '#e6f4ea',
          100: '#c8e6d0',
          200: '#a5d6b5',
          300: '#7dc49a',
          400: '#5ab47f',
          500: '#2e7d32', // Ghana Green (primary)
          600: '#256427',
          700: '#1b4d1d',
          800: '#123614',
          900: '#091f0b',
        },
        gold: {
          50:  '#fffde7',
          100: '#fff9c4',
          200: '#fff59d',
          300: '#fff176',
          400: '#ffee58',
          500: '#fdd835', // Ghana Gold (accent)
          600: '#f9a825',
          700: '#f57f17',
          800: '#e65100',
          900: '#bf360c',
        },
        // Semantic colours
        danger:  { DEFAULT: '#dc2626', light: '#fee2e2', dark: '#991b1b' },
        warning: { DEFAULT: '#d97706', light: '#fef3c7', dark: '#92400e' },
        success: { DEFAULT: '#16a34a', light: '#dcfce7', dark: '#14532d' },
        info:    { DEFAULT: '#2563eb', light: '#dbeafe', dark: '#1e3a8a' },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 3px 0 rgba(0,0,0,0.1), 0 1px 2px -1px rgba(0,0,0,0.1)',
        'card-hover': '0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -2px rgba(0,0,0,0.1)',
      },
    },
  },
  plugins: [],
}
