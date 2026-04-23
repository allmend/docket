/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./templates/**/*.html",
    "./static/src/**/*.js",
  ],
  theme: {
    extend: {
      colors: {
        primary:        "#10B981",  // emerald-500 — Docket brand
        "base-100":     "#0d1117",  // page background
        "base-200":     "#161b22",  // card / surface
        "base-300":     "#21262d",  // border / divider
        "base-content": "#e6edf3",  // primary text
      },
    },
  },
  plugins: [
    require("@tailwindcss/typography"),
  ],
};
