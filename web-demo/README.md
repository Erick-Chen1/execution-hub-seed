# Execution Hub UI Demo

This folder is a demo-only copy of the UI. It runs without a backend and uses
mock responses from `src/api/mock.ts`. By default, demo mode is enabled.

Run it:
```
cd web-demo
npm install
npm run dev
```

To point at a real backend, set:
- `VITE_DEMO_MODE=false`
- `VITE_API_BASE=http://127.0.0.1:8080`
