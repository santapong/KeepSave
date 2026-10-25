import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { LandingPage } from './pages/LandingPage';
import './index.css';

// This entry intentionally has no application, authentication or API imports.
createRoot(document.getElementById('root')!).render(
  <StrictMode><BrowserRouter><LandingPage landingOnly /></BrowserRouter></StrictMode>,
);
