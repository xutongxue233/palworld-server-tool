require("@rushstack/eslint-patch/modern-module-resolution");

module.exports = {
  root: true,
  env: {
    browser: true,
    es2022: true,
    node: true,
  },
  extends: [
    "eslint:recommended",
    "plugin:vue/vue3-essential",
    "@vue/eslint-config-prettier/skip-formatting",
    "./.eslintrc-auto-import.json",
  ],
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
  },
  rules: {
    "no-unused-vars": [
      "warn",
      { varsIgnorePattern: "^(PalDetail|displayHP|percentageHP)$" },
    ],
    "vue/multi-word-component-names": "off",
    "vue/script-setup-uses-vars": "error",
  },
};
