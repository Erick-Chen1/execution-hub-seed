import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? './' : '/',
  plugins: [react()],
  server: {
    proxy: {
      '/v1': 'http://localhost:8080'
    }
  }
}));
