module.exports = {
  content: ["./internal/httpapi/web/templates/admin.html"],
  darkMode: "media",
  theme: {
    extend: {
      colors: {
        proofline: {
          ink: "#17202a",
          "ink-soft": "#475569",
          night: "#140f22",
          shell: "#1d162c",
          surface: "#261d3a",
          "surface-soft": "#302446",
          panel: "#ffffff",
          field: "#f8fafc",
          line: "#d5dbe3",
          action: "#4c1d95",
          "action-soft": "#7c3aed",
          ok: "#15803d",
          warn: "#b45309",
          danger: "#b91c1c",
        },
      },
      boxShadow: {
        panel: "0 1px 2px rgb(15 23 42 / 0.06)",
      },
    },
  },
};
