// ESLint flat config. Next.js 16 removed `next lint`, so linting runs through the
// ESLint CLI directly (`npm run lint`). core-web-vitals promotes the rules that
// affect Core Web Vitals from warnings to errors.
import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts"]),
]);

export default eslintConfig;
