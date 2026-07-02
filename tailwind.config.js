/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./templates/**/*.html",
    "./static/src/**/*.js",
  ],
  theme: {
    extend: {
      colors: {
        base: {
          50:  "#ffffff",
          100: "#0d1117", // page
          150: "#10151c", // sunken (input recess)
          200: "#161b22", // surface (card)
          250: "#1c222b", // raised (hover, popover)
          300: "#21262d", // border / divider
          350: "#2a313b", // strong border
          400: "#30363d", // accent border / focus base
          500: "#484f58", // disabled text
          600: "#6e7681", // placeholder
          700: "#8b949e", // tertiary text
          800: "#b1bac4", // secondary text
          900: "#e6edf3", // primary text
          950: "#f0f6fc", // emphasis text
        },
        primary: {
          DEFAULT: "#10b981",
          300: "#6ee7b7",
          400: "#34d399",
          500: "#10b981",
          600: "#059669",
          700: "#047857",
          900: "#064e3b",
        },
        info:    { DEFAULT: "#58a6ff", muted: "#1f3a5f" },
        warn:    { DEFAULT: "#d29922", muted: "#3d2e0a" },
        danger:  { DEFAULT: "#f85149", muted: "#4a1a1a" },
        success: { DEFAULT: "#10b981", muted: "#0f3a2c" },
        code:    { DEFAULT: "#fb923c", muted: "#3a2412" },
        p0: "#f85149",
        p1: "#ff8c42",
        p2: "#d29922",
        p3: "#58a6ff",
        p4: "#6e7681",
        track: {
          violet: "#8957e5",
          pink:   "#db61a2",
          teal:   "#39c5cf",
          lime:   "#a3e635",
          slate:  "#768390",
        },
      },
      fontFamily: {
        sans: ['"IBM Plex Sans"', "system-ui", "sans-serif"],
        mono: ['"IBM Plex Mono"', "ui-monospace", "monospace"],
      },
      borderRadius: {
        DEFAULT: "4px",
        sm: "3px",
        md: "5px",
        lg: "7px",
      },
      boxShadow: {
        lift:  "0 1px 0 0 rgba(255,255,255,.04) inset, 0 1px 2px rgba(0,0,0,.4)",
        sunk:  "inset 0 1px 0 rgba(0,0,0,.4)",
        pop:   "0 8px 24px rgba(0,0,0,.5), 0 1px 0 rgba(255,255,255,.04) inset",
        focus: "0 0 0 1px #10b981, 0 0 0 4px rgba(16,185,129,.18)",
      },
      letterSpacing: {
        tightish: "-0.011em",
      },
      // v0.3 type scale — lifted one step from v0.2 for legibility on large low-DPI monitors
      fontSize: {
        display: ["56px", { lineHeight: "56px", letterSpacing: "-0.022em" }],
        h1:      ["32px", { lineHeight: "36px", letterSpacing: "-0.014em" }],
        h2:      ["24px", { lineHeight: "32px", letterSpacing: "-0.010em" }],
        h3:      ["17px", { lineHeight: "24px" }],
        body:    ["15px", { lineHeight: "22px" }],
        small:   ["14px", { lineHeight: "20px" }],
        micro:   ["13px", { lineHeight: "16px" }],
        label:   ["13px", { lineHeight: "16px", letterSpacing: "0.18em" }],
      },
    },
  },
  safelist: [
    // dynamically added by JS for refinement active row and other dynamic states
    "border-info", "ring-1", "ring-info/20", "bg-base-250", "border-l-2", "-ml-px",
    "border-p0", "border-p1", "border-p2", "border-p3",
    "bg-p0/10", "bg-p1/10", "bg-p2/10", "bg-p3/10",
    // returned by the priorityColor template func in Go
    "bg-p0", "bg-p1", "bg-p2", "bg-p3", "bg-base-500",
  ],
  plugins: [
    require("@tailwindcss/forms"),
    require("@tailwindcss/typography"),
  ],
};
