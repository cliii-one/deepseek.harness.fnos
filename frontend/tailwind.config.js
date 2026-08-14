/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        fnos: {
          blue: '#1669ff',
          'blue-hover': '#0052e0',
          bg: '#f4f6fb',
          sidebar: '#f7f9fc',
          card: '#ffffff',
        }
      }
    },
  },
  plugins: [],
}
