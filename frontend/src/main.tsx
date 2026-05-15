import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { migrateLegacyJWTKey } from './api/client';
import './index.css';

// FU 0h: copy any legacy `jwt` / `auth_token` localStorage entries into the
// canonical `keepsave_token` key before the app reads auth state.
migrateLegacyJWTKey();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
