import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { login as apiLogin } from '../api/client';
import type { User } from '../types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';


interface LoginPageProps {
  onLogin: (user: User, token: string) => void;
}

export function LoginPage({ onLogin }: LoginPageProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const resp = await apiLogin(email, password);
      onLogin(resp.user, resp.token);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative min-h-screen flex items-center justify-center bg-background overflow-hidden">
      {/* Background orbs */}
      <div className="absolute -top-[20%] -right-[10%] w-[600px] h-[600px] rounded-full bg-[radial-gradient(circle,rgba(79,70,229,0.04)_0%,transparent_70%)] pointer-events-none" />
      <div className="absolute -bottom-[20%] -left-[10%] w-[500px] h-[500px] rounded-full bg-[radial-gradient(circle,rgba(139,92,246,0.03)_0%,transparent_70%)] pointer-events-none" />

      <div className="relative z-10 w-full max-w-[420px] px-5">
        {/* Branding */}
        <div className="text-center mb-8">
          <div className="inline-flex mb-4">
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
              <rect width="40" height="40" rx="10" fill="#6366f1" />
              <path d="M20 10a6 6 0 0 0-6 6v2h-1a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1v-2a6 6 0 0 0-6-6zm-3 8v-2a3 3 0 1 1 6 0v2h-6zm4 5.7V26a1 1 0 1 1-2 0v-2.3a1.5 1.5 0 1 1 2 0z" fill="#fff"/>
            </svg>
          </div>
          <h1 className="text-3xl font-extrabold text-foreground tracking-tight">KeepSave</h1>
          <p className="text-sm text-muted-foreground mt-1">Secure Environment Variable Storage</p>
        </div>

        {/* Login Card */}
        <Card className="shadow-lg">
          <CardHeader className="text-center pb-2">
            <CardTitle className="text-xl font-bold">Welcome back</CardTitle>
            <CardDescription>Sign in to manage your secrets</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              {error && (
                <div className="bg-destructive/10 text-destructive px-3.5 py-2.5 rounded-md text-sm border border-destructive/20">
                  {error}
                </div>
              )}

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="login-email">Email</Label>
                <Input
                  id="login-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  placeholder="you@company.com"
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="login-password">Password</Label>
                <Input
                  id="login-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={8}
                  placeholder="Enter your password"
                />
              </div>

              <Button
                type="submit"
                disabled={loading}
                className="mt-1 w-full bg-gradient-to-br from-indigo-500 to-violet-500 text-white shadow-[0_2px_12px_rgba(99,102,241,0.3)] hover:opacity-90"
              >
                {loading ? 'Signing in...' : 'Sign In'}
              </Button>
            </form>

            {/* Divider */}
            <div className="flex items-center gap-3 my-5">
              <span className="flex-1 h-px bg-border" />
              <span className="text-xs text-muted-foreground">or</span>
              <span className="flex-1 h-px bg-border" />
            </div>

            <p className="text-sm text-center text-muted-foreground">
              Don&apos;t have an account?{' '}
              <Link to="/register" className="text-primary font-medium hover:underline">Register</Link>
            </p>
          </CardContent>
        </Card>

        <p className="text-xs text-muted-foreground text-center mt-6">
          Encrypted at rest with AES-256-GCM
        </p>
      </div>
    </div>
  );
}
