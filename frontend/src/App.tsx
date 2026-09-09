import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useTheme } from './hooks/useTheme';
import { Layout } from './components/Layout';
import { ErrorBoundary } from './components/ErrorBoundary';
import { FeedbackButton } from './components/FeedbackButton';
import { Toaster } from './components/ui/toaster';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { LandingPage } from './pages/LandingPage';

// Authenticated routes are code-split so heavy pages (and recharts, which
// only the admin dashboard uses) load on demand instead of in the initial
// bundle. Login/Register stay eager — they are the logged-out first paint.
const ProjectsPage = lazy(() => import('./pages/ProjectsPage').then((m) => ({ default: m.ProjectsPage })));
const ProjectDetailPage = lazy(() => import('./pages/ProjectDetailPage').then((m) => ({ default: m.ProjectDetailPage })));
const OrganizationsPage = lazy(() => import('./pages/OrganizationsPage').then((m) => ({ default: m.OrganizationsPage })));
const OrganizationManagePage = lazy(() => import('./pages/OrganizationManagePage').then((m) => ({ default: m.OrganizationManagePage })));
const TemplatesPage = lazy(() => import('./pages/TemplatesPage').then((m) => ({ default: m.TemplatesPage })));
const AdminDashboardPage = lazy(() => import('./pages/AdminDashboardPage').then((m) => ({ default: m.AdminDashboardPage })));
const HelpPage = lazy(() => import('./pages/HelpPage').then((m) => ({ default: m.HelpPage })));
const MCPHubPage = lazy(() => import('./pages/MCPHubPage').then((m) => ({ default: m.MCPHubPage })));
const OAuthClientsPage = lazy(() => import('./pages/OAuthClientsPage').then((m) => ({ default: m.OAuthClientsPage })));
const ApplicationDashboardPage = lazy(() => import('./pages/ApplicationDashboardPage').then((m) => ({ default: m.ApplicationDashboardPage })));
const ApplicationSettingsPage = lazy(() => import('./pages/ApplicationSettingsPage').then((m) => ({ default: m.ApplicationSettingsPage })));
const AIIntelligencePage = lazy(() => import('./pages/AIIntelligencePage').then((m) => ({ default: m.AIIntelligencePage })));

function RouteFallback() {
  return (
    <div className="cz-page" style={{ display: 'grid', placeItems: 'center', minHeight: '60vh' }}>
      <div
        className="cz-faint"
        style={{ fontFamily: 'var(--font-mono)', fontSize: 11, letterSpacing: '0.16em', textTransform: 'uppercase' }}
      >
        Loading…
      </div>
    </div>
  );
}

export default function App() {
  useTheme();
  const auth = useAuth();

  if (!auth.authenticated) {
    return (
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<LandingPage />} />
          <Route path="/login" element={<LoginPage onLogin={auth.login} />} />
          <Route path="/register" element={<RegisterPage onLogin={auth.login} />} />
          <Route path="*" element={<LoginPage onLogin={auth.login} />} />
        </Routes>
        <Toaster />
      </BrowserRouter>
    );
  }

  return (
    <BrowserRouter>
      <Layout user={auth.user} onLogout={auth.logout}>
        <ErrorBoundary>
          <Suspense fallback={<RouteFallback />}>
            <Routes>
              <Route path="/" element={<ProjectsPage />} />
              <Route path="/projects/:id/*" element={<ProjectDetailPage />} />
              <Route path="/organizations" element={<OrganizationsPage />} />
              <Route path="/organizations/:id" element={<OrganizationManagePage />} />
              <Route path="/templates" element={<TemplatesPage />} />
              <Route path="/mcp-hub" element={<MCPHubPage />} />
              <Route path="/oauth-clients" element={<OAuthClientsPage />} />
              <Route path="/applications" element={<ApplicationDashboardPage />} />
              <Route path="/applications/settings" element={<ApplicationSettingsPage />} />
              <Route path="/ai/*" element={<AIIntelligencePage />} />
              <Route path="/admin/*" element={<AdminDashboardPage />} />
              <Route path="/help" element={<HelpPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </Layout>
      <FeedbackButton />
      <Toaster />
    </BrowserRouter>
  );
}
