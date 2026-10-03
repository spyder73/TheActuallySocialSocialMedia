import { useState } from "react";
import type { FormEvent } from "react";
import { useNavigate, Link } from "react-router-dom";
import { useAuth, ApiError } from "../auth/AuthContext.js";
import { card, input, btnPrimary } from "../lib/ui.js";

export default function RegisterPage() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await register(email, username, password);
      navigate("/");
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Registration failed"
      );
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className={`${card} w-full max-w-sm`}>
        <h1 className="mb-6 text-xl font-semibold">Registrieren</h1>
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <input
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={input}
            required
          />
          <input
            type="text"
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className={input}
            required
          />
          <input
            type="password"
            placeholder="Password (at least 8 characters)"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={input}
            required
          />
          {error && <p className="text-sm text-red-600">{error}</p>}
          <button type="submit" className={btnPrimary}>
            Create account
          </button>
        </form>
        <p className="mt-4 text-sm text-gray-500">
          Already have an account?{" "}
          <Link to="/login" className="font-medium text-black hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
