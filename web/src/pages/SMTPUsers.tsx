// web/src/pages/SMTPUsers.tsx
import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api';
import type { SMTPUser } from '@/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Users, Plus, Trash2, KeyRound, Eye, EyeOff, CopyIcon } from 'lucide-react';

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

export function SMTPUsers() {
  const [users, setUsers] = useState<SMTPUser[]>([]);
  const [loading, setLoading] = useState(true);

  // Add user dialog
  const [showAdd, setShowAdd] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [adding, setAdding] = useState(false);

  // Reset password dialog
  const [resetUser, setResetUser] = useState<SMTPUser | null>(null);
  const [resetPassword, setResetPassword] = useState('');
  const [resetting, setResetting] = useState(false);

  // Delete confirmation dialog
  const [deleteUser, setDeleteUser] = useState<SMTPUser | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [visiblePasswords, setVisiblePasswords] = useState<Set<number>>(new Set());

  function togglePasswordVisibility(id: number) {
    setVisiblePasswords((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const fetchUsers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.get<SMTPUser[]>('/smtp-users');
      setUsers(data);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to load SMTP users';
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!newUsername.trim() || !newPassword.trim()) return;
    try {
      setAdding(true);
      await api.post('/smtp-users', {
        username: newUsername.trim(),
        password: newPassword,
      });
      toast.success(`User "${newUsername.trim()}" created`);
      setShowAdd(false);
      setNewUsername('');
      setNewPassword('');
      fetchUsers();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to create user';
      toast.error(msg);
    } finally {
      setAdding(false);
    }
  }

  async function handleDelete() {
    if (!deleteUser) return;
    try {
      setDeleting(true);
      await api.del(`/smtp-users/${deleteUser.id}`);
      toast.success(`User "${deleteUser.username}" deleted`);
      setDeleteUser(null);
      fetchUsers();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to delete user';
      toast.error(msg);
    } finally {
      setDeleting(false);
    }
  }

  async function handleResetPassword(e: React.FormEvent) {
    e.preventDefault();
    if (!resetUser || !resetPassword.trim()) return;
    try {
      setResetting(true);
      await api.put(`/smtp-users/${resetUser.id}/password`, {
        password: resetPassword,
      });
      toast.success(`Password updated for "${resetUser.username}"`);
      setResetUser(null);
      setResetPassword('');
      fetchUsers();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to reset password';
      toast.error(msg);
    } finally {
      setResetting(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">SMTP Users</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
            Manage users authorized to send mail via port 587 (submission)
          </p>
        </div>
        <Button size="sm" onClick={() => setShowAdd(true)}>
          <Plus className="h-4 w-4 mr-1.5" />
          Add User
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
          Loading...
        </div>
      ) : users.length === 0 ? (
        <Card className="dark:bg-slate-800">
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <Users className="h-10 w-10 text-slate-300 dark:text-slate-600 mb-3" />
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              No SMTP users configured. Add a user to enable authenticated submission on port 587.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card className="dark:bg-slate-800">
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-24">Status</TableHead>
                  <TableHead>Username</TableHead>
                  <TableHead>Password</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell>
                      <Badge variant={user.active ? 'default' : 'secondary'}>
                        {user.active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm font-medium">{user.username}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <span className="text-sm font-mono text-slate-600 dark:text-slate-400">
                          {user.password
                            ? visiblePasswords.has(user.id) ? user.password : '••••••••'
                            : <span className="italic text-slate-400">—</span>
                          }
                        </span>
                        {user.password && (
                          <>
                            <button
                              type="button"
                              onClick={() => togglePasswordVisibility(user.id)}
                              className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                            >
                              {visiblePasswords.has(user.id)
                                ? <EyeOff className="h-3.5 w-3.5" />
                                : <Eye className="h-3.5 w-3.5" />
                              }
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                navigator.clipboard.writeText(user.password!).then(() => toast.success('Password copied'));
                              }}
                              className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                            >
                              <CopyIcon className="h-3.5 w-3.5" />
                            </button>
                          </>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs text-slate-500 dark:text-slate-400 whitespace-nowrap">
                      {formatTime(user.created_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setResetUser(user);
                            setResetPassword('');
                          }}
                        >
                          <KeyRound className="h-3.5 w-3.5 mr-1" />
                          Reset Password
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-950"
                          onClick={() => setDeleteUser(user)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Add User Dialog */}
      <Dialog open={showAdd} onOpenChange={setShowAdd}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add SMTP User</DialogTitle>
            <DialogDescription>
              Create a new user for authenticated SMTP submission on port 587.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleAdd}>
            <div className="space-y-4 py-2">
              <div className="space-y-2">
                <Label htmlFor="add-username">Username</Label>
                <Input
                  id="add-username"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="user@domain.com"
                  required
                  autoFocus
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="add-password">Password</Label>
                <Input
                  id="add-password"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Minimum 8 characters"
                  minLength={8}
                  required
                />
              </div>
            </div>
            <DialogFooter className="mt-4">
              <Button type="button" variant="outline" onClick={() => setShowAdd(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={adding || !newUsername.trim() || newPassword.length < 8}>
                {adding ? 'Creating...' : 'Create User'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Reset Password Dialog */}
      <Dialog open={!!resetUser} onOpenChange={(open) => !open && setResetUser(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reset Password</DialogTitle>
            <DialogDescription>
              Set a new password for <strong>{resetUser?.username}</strong>.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleResetPassword}>
            <div className="space-y-4 py-2">
              <div className="space-y-2">
                <Label htmlFor="reset-password">New Password</Label>
                <Input
                  id="reset-password"
                  type="password"
                  value={resetPassword}
                  onChange={(e) => setResetPassword(e.target.value)}
                  placeholder="Minimum 8 characters"
                  minLength={8}
                  required
                  autoFocus
                />
              </div>
            </div>
            <DialogFooter className="mt-4">
              <Button type="button" variant="outline" onClick={() => setResetUser(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={resetting || resetPassword.length < 8}>
                {resetting ? 'Updating...' : 'Update Password'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deleteUser} onOpenChange={(open) => !open && setDeleteUser(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete SMTP User</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete <strong>{deleteUser?.username}</strong>?
              This action cannot be undone. If this is the last user, the submission port (587) will be disabled.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteUser(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
