import { createRoot } from 'react-dom/client';
import { TracewayProvider } from '@tracewayapp/react';
import { App } from './App.jsx';

createRoot(document.getElementById('root')).render(
  <TracewayProvider connectionString={import.meta.env.VITE_TRACEWAY_CONNECTION_STRING} options={{ version: '2026.10.3' }}>
    <App />
  </TracewayProvider>,
);
