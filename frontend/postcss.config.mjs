// Named rather than an anonymous object literal: a bare default export gives the
// module no inferable name in stack traces or tooling output.
const config = {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};

export default config;
