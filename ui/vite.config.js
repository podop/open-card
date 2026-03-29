import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8443',
      '/v1': 'http://localhost:8443',
      '/hsm-admin': 'http://localhost:8086',
    },
  },
  build: {
    outDir: 'dist',
  },
});