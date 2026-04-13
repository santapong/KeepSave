import { useState, useCallback, useEffect } from 'react';
import { isAuthenticated, setToken, clearToken } from '../api/client';
import type { User } from '../types';

const USER_KEY = 'keepsave_user';

export function useAuth() {
  const [user, setUser] = useState<User | null>(() => {
    const stored = localStorage.getItem(USER_KEY);
    return stored ? JSON.parse(stored) : null;
  });
  const [authenticated, setAuthenticated] = useState(isAuthenticated);

  useEffect(() => {
    setAuthenticated(isAuthenticated());
  }, [user]);

  // Listen for session-expired events dispatched by the API client on 401
  useEffect(() => {
    const handleSessionExpired = () => {
      setUser(null);
      setAuthenticated(false);
    };

    window.addEventListener('keepsave:session-expired', handleSessionExpired);
    return () => {
      window.removeEventListener('keepsave:session-expired', handleSessionExpired);
    };
  }, []);

  // Periodically check if the token is still valid
  useEffect(() => {
    const interval = setInterval(() => {
      if (!isAuthenticated()) {
        clearToken();
        localStorage.removeItem(USER_KEY);
        setUser(null);
        setAuthenticated(false);
      }
    }, 60_000);
    return () => clearInterval(interval);
  }, []);

  const handleLogin = useCallback((userData: User, token: string) => {
    setToken(token);
    localStorage.setItem(USER_KEY, JSON.stringify(userData));
    setUser(userData);
    setAuthenticated(true);
  }, []);

  const handleLogout = useCallback(() => {
    clearToken();
    localStorage.removeItem(USER_KEY);
    setUser(null);
    setAuthenticated(false);
  }, []);

  return { user, authenticated, login: handleLogin, logout: handleLogout };
}
