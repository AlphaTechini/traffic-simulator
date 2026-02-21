/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        primary: '#0066cc',
        secondary: '#0052a3',
        accent: '#27c93f',
        danger: '#ff5f56',
        warning: '#ffbd2e',
      },
    },
  },
  plugins: [],
};
