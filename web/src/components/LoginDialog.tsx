import { useState } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { setCredentials, api } from '../api';
import { Mail } from 'lucide-react';
import type { StatusResponse } from '../types';

interface LoginDialogProps {
  onLogin: () => void;
}

export function LoginDialog({ onLogin }: LoginDialogProps) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      setCredentials(username, password);
      await api.get<StatusResponse>('/status');
      onLogin();
    } catch {
      setError('Invalid username or password');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative min-h-screen w-full overflow-hidden flex items-center justify-center">
      {/* Background image with slow Ken Burns zoom */}
      <div
        className="absolute inset-0 bg-cover bg-center animate-ken-burns"
        style={{ backgroundImage: 'url(/login-bg.png)' }}
      />
      {/* Dark overlay for readability */}
      <div className="absolute inset-0 bg-black/50" />

      {/* Login card */}
      <div className="relative z-10 w-full max-w-sm mx-4 rounded-xl border border-slate-700/50 bg-slate-900/80 backdrop-blur-xl shadow-2xl p-6">
        <div className="flex items-center gap-2 mb-4">
          <Mail className="text-sky-500 w-5 h-5" />
          <span className="font-semibold text-lg text-white">SignPost</span>
        </div>
        <h2 className="text-xl font-semibold text-white mb-1">Sign in</h2>
        <p className="text-sm text-slate-400 mb-6">Enter your admin credentials to continue.</p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="username" className="text-slate-300">Username</Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              required
              className="bg-slate-800/50 border-slate-600 text-white placeholder:text-slate-500"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="password" className="text-slate-300">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
              className="bg-slate-800/50 border-slate-600 text-white placeholder:text-slate-500"
            />
          </div>
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? 'Signing in\u2026' : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  );
}
