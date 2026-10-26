import React from 'react';
import ReactDOM from 'react-dom/client';
import { I18nProvider } from '@lingui/react';
import { i18n } from './i18n';
import { App } from './App';
import './index.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider i18n={i18n}>
      <App />
    </I18nProvider>
  </React.StrictMode>
);
