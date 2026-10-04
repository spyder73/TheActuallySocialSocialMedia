import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import {
  Link,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
} from "react-router-dom";
import "./pilot.css";
import { PhaseIdentity, PhaseMark } from "../components/PhaseIdentity.js";

type User = {
  id: string;
  username: string;
  email?: string;
  displayName: string;
  bio?: string;
  role?: string;
};
type Post = {
  id: string;
  text: string;
  visibility: "public" | "close_friends";
  createdAt: string;
  author: User;
  parentPostId?: string | null;
  commentCount: number;
};
type Comment = { id: string; text: string; createdAt: string; author: User };
type Conversation = { id: string; name?: string | null; members: User[] };
type Message = { id: string; text: string; createdAt: string; sender: User };
type Relationships = {
  following: string[];
  closeFriends: string[];
  blocked: string[];
};

class HttpError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    if (response.status === 401 && !path.startsWith("/auth/"))
      window.dispatchEvent(new Event("tassm:unauthorized"));
    const msg =
      typeof payload.error === "string"
        ? payload.error
        : response.statusText || "The request failed.";
    throw new HttpError(msg, response.status);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
const initials = (user: User) =>
  (user.displayName || user.username)
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((x) => x[0])
    .join("")
    .toUpperCase();
const when = (value: string) =>
  new Intl.DateTimeFormat("en-GB", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
let capturedFragmentToken: { path: string; token: string } | undefined;
function readAuthToken() {
  const path = window.location.pathname;
  if (path !== "/register" && path !== "/reset") return "";
  const token = new URLSearchParams(window.location.hash.slice(1)).get("token");
  if (token) {
    capturedFragmentToken = { path, token };
    window.history.replaceState(null, "", `${path}${window.location.search}`);
  }
  return capturedFragmentToken?.path === path
    ? capturedFragmentToken.token
    : "";
}

function Avatar({
  user,
  size = "normal",
}: {
  user: User;
  size?: "normal" | "small";
}) {
  return (
    <span
      className={`pl-avatar ${size === "small" ? "small" : ""}`}
      aria-hidden="true"
    >
      {initials(user)}
    </span>
  );
}
function Notice({
  children,
  tone = "error",
}: {
  children: ReactNode;
  tone?: "error" | "info";
}) {
  return (
    <p
      className={`pl-notice ${tone}`}
      role={tone === "error" ? "alert" : "status"}
    >
      {children}
    </p>
  );
}
function Empty({
  title,
  body,
  children,
}: {
  title: string;
  body: string;
  children?: ReactNode;
}) {
  return (
    <div className="pl-empty">
      <span className="pl-seal" aria-hidden="true">
        //
      </span>
      <p className="pl-kicker">START THE CONVERSATION</p>
      <h2>{title}</h2>
      <p>{body}</p>
      {children}
    </div>
  );
}

export default function PilotApp() {
  const location = useLocation();
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [authError, setAuthError] = useState("");
  useEffect(() => {
    const expire = () => {
      setUser(null);
      navigate("/login", { replace: true });
    };
    window.addEventListener("tassm:unauthorized", expire);
    return () => window.removeEventListener("tassm:unauthorized", expire);
  }, [navigate]);
  const [theme, setTheme] = useState<"system" | "light" | "dark">(() => {
    try {
      const value = localStorage.getItem("tassm-theme");
      return value === "light" || value === "system" ? value : "dark";
    } catch {
      return "dark";
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem("tassm-theme", theme);
    } catch {
      /* theme remains usable without storage */
    }
  }, [theme]);
  useEffect(() => {
    let active = true;
    api<{ user: User }>("/auth/me")
      .then(({ user: current }) => {
        if (active) setUser(current);
      })
      .catch((error) => {
        if (active && !(error instanceof HttpError && error.status === 401))
          setAuthError(
            "The Phase service is currently unavailable. Please try again later.",
          );
      })
      .finally(() => {
        if (active) setAuthLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);
  const signOut = async () => {
    try {
      await api("/auth/logout", { method: "POST" });
    } catch {
      setAuthError("Could not sign out right now.");
      return;
    }
    setUser(null);
    navigate("/login");
  };
  if (authLoading)
    return (
      <div className="pl-loading" role="status">
        Opening Phase …
      </div>
    );
  const isAuthRoute = ["/login", "/register", "/reset"].includes(
    location.pathname,
  );
  if (!user && !isAuthRoute)
    return <AuthScreen mode="login" error={authError} onSuccess={setUser} />;
  if (!user)
    return (
      <AuthScreen
        key={location.pathname}
        mode={location.pathname.slice(1) as "login" | "register" | "reset"}
        error={authError}
        onSuccess={setUser}
      />
    );
  return (
    <Shell
      user={user}
      theme={theme}
      error={authError}
      onTheme={() =>
        setTheme(
          theme === "system" ? "light" : theme === "light" ? "dark" : "system",
        )
      }
      onSignOut={signOut}
    >
      <Routes>
        <Route path="/" element={<Feed user={user} />} />
        <Route
          path="/post/:id"
          element={<Thread key={location.pathname} user={user} />}
        />
        <Route path="/members" element={<Members user={user} />} />
        <Route
          path="/u/:username"
          element={
            <Profile key={location.pathname} user={user} onUser={setUser} />
          }
        />
        <Route path="/messages" element={<Messages user={user} />} />
        <Route path="*" element={<Feed user={user} />} />
      </Routes>
    </Shell>
  );
}

function AuthScreen({
  mode,
  error: serviceError,
  onSuccess,
}: {
  mode: "login" | "register" | "reset";
  error: string;
  onSuccess: (user: User) => void;
}) {
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(serviceError);
  const [done, setDone] = useState(false);
  const tokenRef = useRef(readAuthToken());
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      if (mode === "login") {
        const result = await api<{ user: User }>("/auth/login", {
          method: "POST",
          body: JSON.stringify({
            email: data.get("email"),
            password: data.get("password"),
          }),
        });
        capturedFragmentToken = undefined;
        onSuccess(result.user);
        navigate("/");
      } else if (mode === "register") {
        const result = await api<{ user: User }>("/auth/register", {
          method: "POST",
          body: JSON.stringify({
            token: tokenRef.current,
            email: data.get("email"),
            username: data.get("username"),
            password: data.get("password"),
            displayName: data.get("displayName"),
          }),
        });
        capturedFragmentToken = undefined;
        onSuccess(result.user);
        navigate("/");
      } else {
        await api("/auth/reset", {
          method: "POST",
          body: JSON.stringify({
            token: tokenRef.current,
            password: data.get("password"),
          }),
        });
        capturedFragmentToken = undefined;
        setDone(true);
      }
    } catch (reason) {
      setError(
        reason instanceof Error ? reason.message : "Please try again.",
      );
    } finally {
      setBusy(false);
    }
  }
  const title =
    mode === "login"
      ? "Good to have you here."
      : mode === "register"
        ? "Your space is ready."
        : "A fresh start.";
  return (
    <main className="pl-auth" data-theme="dark">
      <section className="pl-auth-card">
        <Link to="/login" className="pl-brand">
          <span className="pl-brand-mark"><PhaseMark/></span><strong>Phase</strong>
        </Link>
        <p className="pl-kicker">INDIVIDUAL VOICES. COLLECTIVE STATES.</p>
        <h1>{title}</h1>
        <p className="pl-auth-intro">
          A place to follow people, share ideas, and stay in the conversation.
        </p>
        {error && <Notice>{error}</Notice>}
        {done && (
          <Notice tone="info">
            Your password has been changed. You can now sign in.
          </Notice>
        )}
        {mode !== "login" && !tokenRef.current && (
          <Notice>
            Please open the full invitation or
            recovery link from your administrator.
          </Notice>
        )}
        {!done && (
          <form className="pl-form" onSubmit={submit}>
            {mode === "register" && (
              <>
                <label>
                  Display name
                  <input
                    name="displayName"
                    autoComplete="name"
                    required
                    maxLength={80}
                  />
                </label>
                <label>
                  Username
                  <input
                    name="username"
                    autoComplete="username"
                    required
                    minLength={3}
                    maxLength={30}
                    pattern="[A-Za-z0-9_]+"
                  />
                </label>
              </>
            )}
            {mode !== "reset" && (
              <label>
                Email address
                <input
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                />
              </label>
            )}
            {mode !== "reset" && (
              <label>
                Password
                <input
                  name="password"
                  type="password"
                  autoComplete={
                    mode === "login" ? "current-password" : "new-password"
                  }
                  required
                  minLength={mode === "register" ? 12 : undefined}
                />
              </label>
            )}
            {mode === "reset" && (
              <label>
                New password
                <input
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={12}
                />
              </label>
            )}
            <button
              className="pl-button primary"
              disabled={
                busy ||
                ((mode === "register" || mode === "reset") && !tokenRef.current)
              }
            >
              {busy
                ? "One moment …"
                : mode === "login"
                  ? "Sign in"
                  : mode === "register"
                    ? "Create account"
                    : "Change password"}
            </button>
          </form>
        )}
        <div className="pl-auth-links">
          {mode !== "login" && <Link to="/login">Back to sign in</Link>}
          {mode === "login" && (
            <Link to="/register">Have an invitation? Create an account</Link>
          )}
          {mode === "login" && <Link to="/reset">Reset password</Link>}
        </div>
        <p className="pl-private-note">
          Small, private, and chronological. Your feed only shows posts from
          your network.
        </p>
      </section>
    </main>
  );
}

function Shell({
  user,
  theme,
  error,
  onTheme,
  onSignOut,
  children,
}: {
  user: User;
  theme: string;
  error: string;
  onTheme: () => void;
  onSignOut: () => void;
  children: ReactNode;
}) {
  const location = useLocation();
  const nav = [
    { to: "/", label: "Feed", icon: "⌂" },
    { to: "/members", label: "Members", icon: "◎" },
    { to: "/messages", label: "Conversations", icon: "↗" },
    { to: `/u/${user.username}`, label: "My profile", icon: "◌" },
  ];
  return (
    <div className="pl-app" data-theme={theme}>
      <div className="pl-layout">
        <aside className="pl-sidebar">
          <Link to="/" className="pl-brand">
            <span className="pl-brand-mark"><PhaseMark/></span><strong>Phase</strong>
          </Link>
          <p className="pl-sidebar-caption">
            POSTS FROM YOUR NETWORK
          </p>
          <nav className="pl-nav" aria-label="Main navigation">
            {nav.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                className={
                  location.pathname === item.to ||
                  (item.to === "/" && location.pathname.startsWith("/post/"))
                    ? "active"
                    : ""
                }
              >
                <span aria-hidden="true">{item.icon}</span>
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="pl-sidebar-note">
            <span className="pl-sidebar-phase-mark" aria-hidden="true"><PhaseMark /></span>
            <p>Posts from your network, in order.</p>
          </div>
          <div className="pl-sidebar-bottom">
            <Link to={`/u/${user.username}`} className="pl-self">
              <Avatar user={user} size="small" />
              <span>
                <strong>{user.displayName}</strong>
                <small>@{user.username}</small>
              </span>
            </Link>
            <button className="pl-text-button" onClick={onSignOut}>
              Sign out
            </button>
          </div>
        </aside>
        <main className="pl-main">
          <header className="pl-mobile-head">
            <Link to="/" className="pl-brand">
              <span className="pl-brand-mark"><PhaseMark/></span><strong>Phase</strong>
            </Link>
            <div className="pl-mobile-head-actions">
              <button
                className="pl-icon-button"
                onClick={onTheme}
                aria-label={`Change colour theme; currently ${theme === "system" ? "System" : theme === "light" ? "Light" : "Dark"}`}
              >
                ◐
              </button>
              <button className="pl-text-button" onClick={onSignOut}>
                Sign out
              </button>
            </div>
          </header>
          <div className="pl-main-toolbar">
            <span className="pl-kicker">THE FEED</span>
            <button className="pl-theme" onClick={onTheme}>
              ◐{" "}
              <span>
                {{ system: "System", light: "Light", dark: "Dark" }[theme]}
              </span>
            </button>
          </div>
          {error && (
            <div className="pl-shell-error">
              <Notice>{error}</Notice>
            </div>
          )}
          {children}
        </main>
        <aside className="pl-rail">
          <div className="pl-rail-card">
            <p className="pl-kicker">WHAT TO EXPECT</p>
            <h2>Your network.</h2>
            <p>
              Posts appear in chronological order across the network. No rankings and
              no fabricated activity.
            </p>
          </div>
          <div className="pl-rail-foot">
            Invitation-only access
            <br />
            Phase · {new Date().getFullYear()}
          </div>
        </aside>
      </div>
      <nav className="pl-mobile-nav" aria-label="Main navigation">
        {nav.map((item) => (
          <Link
            key={item.to}
            aria-label={item.label}
            to={item.to}
            className={location.pathname === item.to ? "active" : ""}
          >
            <span aria-hidden="true">{item.icon}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}

function PageHeading({
  eyebrow,
  title,
  subtitle,
  action,
}: {
  eyebrow: string;
  title: string;
  subtitle: string;
  action?: ReactNode;
}) {
  return (
    <header className="pl-page-heading">
      <div>
        <p className="pl-kicker">{eyebrow}</p>
        <h1>{title}</h1>
        <p className="pl-subtitle">{subtitle}</p>
      </div>
      {title === "Feed" && <PhaseIdentity />}
      {action}
    </header>
  );
}

function Feed({ user }: { user: User }) {
  const [posts, setPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [composer, setComposer] = useState(false);
  const [text, setText] = useState("");
  const [visibility, setVisibility] = useState<"public" | "close_friends">(
    "public",
  );
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  busyRef.current = busy;
  const [mutationError, setMutationError] = useState("");
  const textarea = useRef<HTMLTextAreaElement>(null);
  const load = useCallback(async (cursor?: string) => {
    setLoading(true);
    setError("");
    try {
      const result = await api<{ posts: Post[]; nextCursor: string | null }>(
        `/feed${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`,
      );
      setPosts((previous) =>
        cursor ? [...previous, ...result.posts] : result.posts,
      );
      setNextCursor(result.nextCursor);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load the feed.",
      );
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!composer) return;
    const prior = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    textarea.current?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busyRef.current) setComposer(false);
      if (event.key === "Tab") {
        const focusables = document
          .querySelector<HTMLElement>(".pl-dialog")
          ?.querySelectorAll<HTMLElement>(
            "button:not([disabled]),textarea:not([disabled]),select:not([disabled])",
          );
        if (!focusables?.length) return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      document.body.style.overflow = overflow;
      prior?.focus();
    };
  }, [composer]);
  async function publish(event: FormEvent) {
    event.preventDefault();
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    setMutationError("");
    try {
      const result = await api<{ post: Post }>("/posts", {
        method: "POST",
        body: JSON.stringify({ text: trimmed, visibility }),
      });
      setPosts((previous) => [result.post, ...previous]);
      setText("");
      setComposer(false);
    } catch (reason) {
      setMutationError(
        reason instanceof Error
          ? reason.message
          : "Could not publish the post.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="CHRONOLOGICAL · NO RANKINGS"
        title="Feed"
        subtitle="Posts from your network, in chronological order."
        action={
          <button
            className="pl-button primary"
            onClick={() => setComposer(true)}
          >
            ＋ <span>Post</span>
          </button>
        }
      />
      <section className="pl-feed" aria-label="Posts from your network">
        {error && (
          <div className="pl-feed-state">
            <Notice>{error}</Notice>
            <button className="pl-button quiet" onClick={() => void load()}>
              Try again
            </button>
          </div>
        )}
        {loading && !posts.length && (
          <p className="pl-inline-loading" role="status">
            Loading posts …
          </p>
        )}
        {!loading && !error && posts.length === 0 && (
          <Empty
            title="No posts here yet."
            body="No posts yet. Start the conversation."
          >
            <button
              className="pl-button primary"
              onClick={() => setComposer(true)}
            >
              ＋ Write the first post
            </button>
          </Empty>
        )}
        {posts.map((post) => (
          <PostCard key={post.id} post={post} currentUser={user} />
        ))}
        {nextCursor && (
          <div className="pl-load-more">
            <button
              className="pl-button quiet"
              disabled={loading}
              onClick={() => void load(nextCursor)}
            >
              {loading ? "Loading …" : "Load older posts"}
            </button>
          </div>
        )}
      </section>
      {composer && (
        <div
          className="pl-overlay"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget && !busy)
              setComposer(false);
          }}
        >
          <section
            className="pl-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="compose-title"
          >
            <header className="pl-dialog-head">
              <div>
                <p className="pl-kicker">A THOUGHT FROM YOU</p>
                <h2 id="compose-title">What’s on your mind?</h2>
              </div>
              <button
                className="pl-icon-button"
                aria-label="Close"
                onClick={() => setComposer(false)}
              >
                ×
              </button>
            </header>
            <form onSubmit={publish}>
              <label className="pl-field-label" htmlFor="compose-text">
                Your post
              </label>
              <textarea
                id="compose-text"
                ref={textarea}
                value={text}
                maxLength={5000}
                onChange={(event) => setText(event.target.value)}
                placeholder="Write a post …"
                required
              />
              <div className="pl-compose-options">
                <label>
                  Visibility
                  <select
                    value={visibility}
                    onChange={(event) =>
                      setVisibility(event.target.value as typeof visibility)
                    }
                  >
                    <option value="public">All members</option>
                    <option value="close_friends">Close friends</option>
                  </select>
                </label>
                <span>{text.length}/5000</span>
              </div>
              {mutationError && <Notice>{mutationError}</Notice>}
              <div className="pl-dialog-actions">
                <button
                  type="button"
                  className="pl-button quiet"
                  disabled={busy}
                  onClick={() => setComposer(false)}
                >
                  Cancel
                </button>
                <button
                  className="pl-button primary"
                  disabled={busy || !text.trim()}
                >
                  {busy ? "Publishing …" : "Publish post"}
                </button>
              </div>
            </form>
          </section>
        </div>
      )}
    </>
  );
}
function PostCard({ post, currentUser }: { post: Post; currentUser: User }) {
  const [notice, setNotice] = useState("");
  async function report() {
    const reason = window
      .prompt("Why are you reporting this post?")
      ?.trim();
    if (!reason) return;
    try {
      await api("/reports", {
        method: "POST",
        body: JSON.stringify({ targetType: "post", targetId: post.id, reason }),
      });
      setNotice("Thanks. Your report has been submitted.");
    } catch (error) {
      setNotice(
        error instanceof Error
          ? error.message
          : "Could not submit the report.",
      );
    }
  }
  async function remove() {
    if (!window.confirm("Permanently delete this post and its replies?"))
      return;
    try {
      await api(`/posts/${encodeURIComponent(post.id)}`, { method: "DELETE" });
      window.location.reload();
    } catch (error) {
      setNotice(
        error instanceof Error
          ? error.message
          : "Could not delete the post.",
      );
    }
  }
  return (
    <article className="pl-post">
      <Link to={`/u/${post.author.username}`} className="pl-post-avatar">
        <Avatar user={post.author} />
      </Link>
      <div className="pl-post-content">
        <div className="pl-post-meta">
          <Link to={`/u/${post.author.username}`}>
            <strong>{post.author.displayName}</strong>
            <span>@{post.author.username}</span>
          </Link>
          <time dateTime={post.createdAt}>{when(post.createdAt)}</time>
          {post.visibility === "close_friends" && (
            <span className="pl-visibility">Close friends</span>
          )}
        </div>
        <p className="pl-post-text">{post.text}</p>
        <div className="pl-post-actions">
          <Link
            to={`/post/${post.id}`}
            aria-label={`${post.commentCount} View replies`}
          >
            ↩{" "}
            <span>
              {post.commentCount}{" "}
              {post.commentCount === 1 ? "Reply" : "Replies"}
            </span>
          </Link>
          {post.author.id === currentUser.id ? (
            <>
              <span className="pl-by-you">Your post</span>
              <button onClick={() => void remove()}>Delete</button>
            </>
          ) : (
            <button onClick={() => void report()}>Report</button>
          )}
        </div>
        {notice && <Notice tone="info">{notice}</Notice>}
      </div>
    </article>
  );
}

function Thread({ user }: { user: User }) {
  const { id: postId = "" } = useParams();
  const navigate = useNavigate();
  const [post, setPost] = useState<Post | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    setPost(null);
    setComments([]);
    try {
      const [postResult, commentResult] = await Promise.all([
        api<{ post: Post }>(`/posts/${encodeURIComponent(postId)}`),
        api<{ comments: Comment[] }>(
          `/posts/${encodeURIComponent(postId)}/comments`,
        ),
      ]);
      setPost(postResult.post);
      setComments(commentResult.comments);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load the thread.",
      );
    } finally {
      setLoading(false);
    }
  }, [postId]);
  useEffect(() => {
    void load();
  }, [load]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!text.trim() || busy) return;
    setBusy(true);
    try {
      const result = await api<{ comment: Comment }>(
        `/posts/${encodeURIComponent(postId)}/comments`,
        { method: "POST", body: JSON.stringify({ text: text.trim() }) },
      );
      setComments((previous) => [...previous, result.comment]);
      setText("");
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not publish the reply.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function report() {
    if (!post) return;
    const reason = window
      .prompt("Why are you reporting this post?")
      ?.trim();
    if (!reason) return;
    try {
      await api("/reports", {
        method: "POST",
        body: JSON.stringify({ targetType: "post", targetId: post.id, reason }),
      });
      setError("Thanks. Your report has been submitted.");
    } catch (problem) {
      setError(
        problem instanceof Error
          ? problem.message
          : "Could not submit the report.",
      );
    }
  }
  async function remove() {
    if (
      !post ||
      !window.confirm("Permanently delete this post and its replies?")
    )
      return;
    try {
      await api(`/posts/${encodeURIComponent(post.id)}`, { method: "DELETE" });
      navigate("/");
    } catch (problem) {
      setError(
        problem instanceof Error
          ? problem.message
          : "Could not delete the post.",
      );
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="THREAD"
        title="Post thread"
        subtitle="Replies stay where the conversation began."
      />
      <div className="pl-thread">
        <Link className="pl-back-link" to="/">
          ← Back to the feed
        </Link>
        {loading && (
          <p role="status" className="pl-inline-loading">
            Loading thread …
          </p>
        )}
        {error && <Notice>{error}</Notice>}
        {post && (
          <>
            <article className="pl-thread-root">
              <div className="pl-post-meta">
                <Link to={`/u/${post.author.username}`}>
                  <strong>{post.author.displayName}</strong>
                  <span>@{post.author.username}</span>
                </Link>
                <time>{when(post.createdAt)}</time>
              </div>
              <p className="pl-post-text">{post.text}</p>
              <div className="pl-post-actions">
                {post.author.id === user.id ? (
                  <button onClick={() => void remove()}>Delete post</button>
                ) : (
                  <button onClick={() => void report()}>Report post</button>
                )}
              </div>
            </article>
            <section className="pl-comments" aria-label="Replies">
              {comments.map((comment) => (
                <article key={comment.id} className="pl-comment">
                  <Avatar user={comment.author} size="small" />
                  <div>
                    <div className="pl-post-meta">
                      <Link to={`/u/${comment.author.username}`}>
                        <strong>{comment.author.displayName}</strong>
                        <span>@{comment.author.username}</span>
                      </Link>
                      <time>{when(comment.createdAt)}</time>
                    </div>
                    <p>{comment.text}</p>
                  </div>
                </article>
              ))}
              {comments.length === 0 && (
                <p className="pl-muted">
                  No replies yet. The conversation starts here.
                </p>
              )}
              {comments.length >= 500 && (
                <p className="pl-muted">
                  Showing the first 500 replies.
                </p>
              )}
            </section>
            <form className="pl-reply-form" onSubmit={submit}>
              <label htmlFor="reply-text">Your reply</label>
              <textarea
                id="reply-text"
                value={text}
                onChange={(event) => setText(event.target.value)}
                maxLength={2000}
                required
                placeholder="Reply thoughtfully …"
              />
              <button
                className="pl-button primary"
                disabled={busy || !text.trim()}
              >
                {busy ? "Sending …" : "Replies"}
              </button>
            </form>
          </>
        )}
      </div>
    </>
  );
}

function Members({ user }: { user: User }) {
  const [members, setMembers] = useState<User[]>([]);
  const [relationships, setRelationships] = useState<Relationships>({
    following: [],
    closeFriends: [],
    blocked: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState("");
  const [busyId, setBusyId] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [memberData, relationData] = await Promise.all([
        api<{ users: User[] }>("/members"),
        api<Relationships>("/relationships"),
      ]);
      setMembers(memberData.users.filter((person) => person.id !== user.id));
      setRelationships(relationData);
      setError("");
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load members.",
      );
    } finally {
      setLoading(false);
    }
  }, [user.id]);
  useEffect(() => {
    void load();
  }, [load]);
  const matching = members.filter((person) =>
    `${person.displayName} ${person.username}`
      .toLocaleLowerCase("en-GB")
      .includes(filter.toLocaleLowerCase("en-GB")),
  );
  async function toggle(person: User, kind: "follow" | "close_friend") {
    setBusyId(person.id);
    setError("");
    const list = kind === "follow" ? "following" : "closeFriends";
    const active = relationships[list].includes(person.id);
    try {
      await api(`/relationships/${kind}/${encodeURIComponent(person.id)}`, {
        method: active ? "DELETE" : "PUT",
      });
      setRelationships((current) => ({
        ...current,
        [list]: active
          ? current[list].filter((id) => id !== person.id)
          : [...current[list], person.id],
      }));
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not save the change.",
      );
    } finally {
      setBusyId("");
    }
  }
  async function unblock(id: string) {
    setBusyId(id);
    try {
      await api(`/relationships/block/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      setRelationships((value) => ({
        ...value,
        blocked: value.blocked.filter((item) => item !== id),
      }));
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not unblock this member.",
      );
    } finally {
      setBusyId("");
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="YOUR NETWORK"
        title="People over popularity."
        subtitle="Follow people whose ideas you enjoy reading."
      />
      <div className="pl-members">
        <label className="pl-search">
          Find members
          <input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Name or username"
          />
        </label>
        {error && <Notice>{error}</Notice>}
        {loading && (
          <p role="status" className="pl-inline-loading">
            Loading members …
          </p>
        )}
        {!loading && !error && matching.length === 0 && (
          <Empty
            title={
              filter ? "No one found." : "Your network is growing at its own pace."
            }
            body={
              filter
                ? "Change your search and try again."
                : "There are no more members right now."
            }
          />
        )}
        {matching.map((person) => (
          <article key={person.id} className="pl-member">
            <Avatar user={person} />
            <div className="pl-member-info">
              <Link to={`/u/${person.username}`}>
                <strong>{person.displayName}</strong>
                <span>@{person.username}</span>
              </Link>
              {person.bio && <p>{person.bio}</p>}
            </div>
            <div className="pl-member-actions">
              <button
                className={`pl-button ${relationships.following.includes(person.id) ? "quiet" : "outline"}`}
                disabled={busyId === person.id}
                onClick={() => void toggle(person, "follow")}
              >
                {relationships.following.includes(person.id)
                  ? "Following"
                  : "Follow"}
              </button>
              <button
                className={`pl-button mini ${relationships.closeFriends.includes(person.id) ? "selected" : ""}`}
                disabled={busyId === person.id}
                onClick={() => void toggle(person, "close_friend")}
                aria-label={`${relationships.closeFriends.includes(person.id) ? "Remove from close friends" : "Add to close friends"}: ${person.displayName}`}
                title="Close friends"
              >
                //
              </button>
              <button
                className="pl-button quiet"
                onClick={async () => {
                  const reason = window
                    .prompt("Why are you reporting this member?")
                    ?.trim();
                  if (!reason) return;
                  try {
                    await api("/reports", {
                      method: "POST",
                      body: JSON.stringify({
                        targetType: "user",
                        targetId: person.id,
                        reason,
                      }),
                    });
                    setError("Thanks. Your report has been submitted.");
                  } catch (problem) {
                    setError(
                      problem instanceof Error
                        ? problem.message
                        : "Could not submit the report.",
                    );
                  }
                }}
              >
                Report
              </button>
            </div>
          </article>
        ))}
        {relationships.blocked.length > 0 && (
          <section className="pl-blocked">
            <h2>Blocked accounts</h2>
            {relationships.blocked.map((id) => (
              <div key={id}>
                <code>{id}</code>
                <button
                  className="pl-button quiet"
                  disabled={busyId === id}
                  onClick={() => void unblock(id)}
                >
                  Unblock
                </button>
              </div>
            ))}
          </section>
        )}
      </div>
    </>
  );
}

function Profile({
  user: current,
  onUser,
}: {
  user: User;
  onUser: (user: User) => void;
}) {
  const { pathname } = useLocation();
  const username = decodeURIComponent(pathname.split("/").at(-1) || "");
  const own = username === current.username;
  const [profile, setProfile] = useState<User | null>(null);
  const [posts, setPosts] = useState<Post[]>([]);
  const [relationships, setRelationships] = useState<Relationships>({
    following: [],
    closeFriends: [],
    blocked: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [edit, setEdit] = useState(false);
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    setProfile(own ? current : null);
    setPosts([]);
    try {
      const [memberData, relationData] = await Promise.all([
        api<{ users: User[] }>("/members"),
        api<Relationships>("/relationships"),
      ]);
      const found = own
        ? current
        : (memberData.users.find((person) => person.username === username) ??
          null);
      if (!found) throw new Error("This profile is currently unavailable.");
      setProfile(found);
      setRelationships(relationData);
      const matchingPosts: Post[] = [];
      let cursor: string | null = null;
      do {
        const page: { posts: Post[]; nextCursor: string | null } = await api<{
          posts: Post[];
          nextCursor: string | null;
        }>(`/feed${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ""}`);
        matchingPosts.push(
          ...page.posts.filter((post) => post.author.id === found.id),
        );
        cursor = page.nextCursor;
      } while (cursor);
      setPosts(matchingPosts);
    } catch (reason) {
      setProfile(null);
      setPosts([]);
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load the profile.",
      );
    } finally {
      setLoading(false);
    }
  }, [current, own, username]);
  useEffect(() => {
    void load();
  }, [load]);
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!profile) return;
    setBusy(true);
    const data = new FormData(event.currentTarget);
    try {
      const result = await api<{ user: User }>("/profile", {
        method: "PATCH",
        body: JSON.stringify({
          displayName: data.get("displayName"),
          bio: data.get("bio"),
        }),
      });
      const updated = {
        ...current,
        ...result.user,
        email: current.email,
        role: current.role,
      };
      onUser(updated);
      setProfile(updated);
      setEdit(false);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not save the profile.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function relationship(kind: "follow" | "close_friend" | "block") {
    if (!profile) return;
    const list =
      kind === "follow"
        ? "following"
        : kind === "close_friend"
          ? "closeFriends"
          : "blocked";
    const active = relationships[list].includes(profile.id);
    setBusy(true);
    try {
      await api(`/relationships/${kind}/${encodeURIComponent(profile.id)}`, {
        method: active ? "DELETE" : "PUT",
      });
      await load();
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not save the change.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function unblock(id: string) {
    setBusy(true);
    try {
      await api(`/relationships/block/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      setRelationships((value) => ({
        ...value,
        blocked: value.blocked.filter((item) => item !== id),
      }));
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not unblock this member.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="MEMBER PROFILE"
        title={profile?.displayName ?? "Profile"}
        subtitle={`@${username}`}
      />
      {loading && (
        <p role="status" className="pl-inline-loading">
          Loading profile …
        </p>
      )}
      {error && <Notice>{error}</Notice>}
      {profile && (
        <>
          <section className="pl-profile-card">
            <Avatar user={profile} />
            <div className="pl-profile-main">
              <p className="pl-kicker">MITGLIED IM PRIVATEN NETZWERK</p>
              <h2>{profile.displayName}</h2>
              <p className="pl-handle">@{profile.username}</p>
              {profile.bio && <p className="pl-bio">{profile.bio}</p>}
            </div>
            <div className="pl-profile-actions">
              {own ? (
                <button
                  className="pl-button outline"
                  onClick={() => setEdit((value) => !value)}
                >
                  {edit ? "Close" : "Edit profile"}
                </button>
              ) : (
                <>
                  <button
                    disabled={busy}
                    className="pl-button outline"
                    onClick={() => void relationship("follow")}
                  >
                    {relationships.following.includes(profile.id)
                      ? "Unfollow"
                      : "Follow"}
                  </button>
                  <button
                    disabled={busy}
                    className={`pl-button mini ${relationships.closeFriends.includes(profile.id) ? "selected" : ""}`}
                    onClick={() => void relationship("close_friend")}
                    aria-label="Toggle close-friend status"
                  >
                    //
                  </button>
                  <button
                    disabled={busy}
                    className="pl-button quiet"
                    onClick={() => void relationship("block")}
                  >
                    {relationships.blocked.includes(profile.id)
                      ? "Unblock"
                      : "Block"}
                  </button>
                  <button
                    disabled={busy}
                    className="pl-button quiet"
                    onClick={async () => {
                      const reason = window
                        .prompt("Why are you reporting this member?")
                        ?.trim();
                      if (!reason) return;
                      try {
                        await api("/reports", {
                          method: "POST",
                          body: JSON.stringify({
                            targetType: "user",
                            targetId: profile.id,
                            reason,
                          }),
                        });
                        setError("Thanks. Your report has been submitted.");
                      } catch (problem) {
                        setError(
                          problem instanceof Error
                            ? problem.message
                            : "Could not submit the report.",
                        );
                      }
                    }}
                  >
                    Report
                  </button>
                </>
              )}
            </div>
          </section>
          {edit && (
            <form className="pl-edit-form" onSubmit={save}>
              <label>
                Display name
                <input
                  name="displayName"
                  defaultValue={profile.displayName}
                  maxLength={80}
                  required
                />
              </label>
              <label>
                About me
                <textarea
                  name="bio"
                  defaultValue={profile.bio ?? ""}
                  maxLength={500}
                />
              </label>
              <button className="pl-button primary" disabled={busy}>
                {busy ? "Saving …" : "Save changes"}
              </button>
            </form>
          )}
          <section className="pl-profile-posts">
            <h2>Posts</h2>
            {posts.length
              ? posts.map((post) => (
                  <PostCard key={post.id} post={post} currentUser={current} />
                ))
              : !loading && (
                  <p className="pl-muted">No posts yet.</p>
                )}
          </section>
          {own && relationships.blocked.length > 0 && (
            <section className="pl-blocked pl-own-blocked">
              <h2>Blocked accounts</h2>
              {relationships.blocked.map((id) => (
                <div key={id}>
                  <code>{id}</code>
                  <button
                    className="pl-button quiet"
                    disabled={busy}
                    onClick={() => void unblock(id)}
                  >
                    Unblock
                  </button>
                </div>
              ))}
            </section>
          )}
        </>
      )}
    </>
  );
}

function Messages({ user }: { user: User }) {
  const [items, setItems] = useState<Conversation[]>([]);
  const [selected, setSelected] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [nextMessageCursor, setNextMessageCursor] = useState<string | null>(
    null,
  );
  const [olderLoading, setOlderLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [recipient, setRecipient] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api<{ conversations: Conversation[] }>(
        "/conversations",
      );
      setItems(result.conversations);
      setSelected((value) =>
        value && result.conversations.some((item) => item.id === value)
          ? value
          : (result.conversations[0]?.id ?? ""),
      );
      setError("");
    } catch (reason) {
      setItems([]);
      setSelected("");
      setMessages([]);
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load conversations.",
      );
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const loadMessages = useCallback(async () => {
    const requestedConversation = selected;
    setMessages([]);
    setNextMessageCursor(null);
    if (!selected) return;
    setError("");
    try {
      const result = await api<{
        messages: Message[];
        nextCursor: string | null;
      }>(`/conversations/${encodeURIComponent(selected)}/messages`);
      if (selectedRef.current !== requestedConversation) return;
      setMessages([...result.messages].reverse());
      setNextMessageCursor(result.nextCursor);
    } catch (reason) {
      if (selectedRef.current !== requestedConversation) return;
      setMessages([]);
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load messages.",
      );
    }
  }, [selected]);
  useEffect(() => {
    void loadMessages();
  }, [loadMessages]);
  async function loadOlderMessages() {
    const requestedConversation = selected;
    if (!nextMessageCursor || !selected || olderLoading) return;
    setOlderLoading(true);
    try {
      const result = await api<{
        messages: Message[];
        nextCursor: string | null;
      }>(
        `/conversations/${encodeURIComponent(selected)}/messages?cursor=${encodeURIComponent(nextMessageCursor)}`,
      );
      if (selectedRef.current !== requestedConversation) return;
      setMessages((previous) => [...result.messages.reverse(), ...previous]);
      setNextMessageCursor(result.nextCursor);
    } catch (reason) {
      if (selectedRef.current !== requestedConversation) return;
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not load older messages.",
      );
    } finally {
      setOlderLoading(false);
    }
  }
  async function send(event: FormEvent) {
    const requestedConversation = selected;
    event.preventDefault();
    if (!text.trim() || !selected || busy) return;
    setBusy(true);
    try {
      const result = await api<{ message: Message }>(
        `/conversations/${encodeURIComponent(selected)}/messages`,
        { method: "POST", body: JSON.stringify({ text: text.trim() }) },
      );
      if (selectedRef.current !== requestedConversation) return;
      setMessages((previous) => [...previous, result.message]);
      setText("");
    } catch (reason) {
      if (selectedRef.current !== requestedConversation) return;
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not send the message.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function start(event: FormEvent) {
    event.preventDefault();
    if (!recipient.trim() || busy) return;
    setBusy(true);
    setError("");
    try {
      const directory = await api<{ users: User[] }>("/members");
      const names = [
        ...new Set(
          recipient
            .split(",")
            .map((name) => name.trim().replace(/^@/, "").toLowerCase())
            .filter(Boolean),
        ),
      ];
      const people = names.map((name) =>
        directory.users.find(
          (person) => person.username.toLowerCase() === name,
        ),
      );
      if (!names.length || people.some((person) => !person))
        throw new Error("At least one member could not be found.");
      const members = people as User[];
      const result = await api<{ conversation: Conversation }>(
        "/conversations",
        {
          method: "POST",
          body: JSON.stringify({
            memberIds: members.map((person) => person.id),
            ...(members.length > 1
              ? {
                  name: members
                    .map((person) => person.displayName || person.username)
                    .join(", ")
                    .slice(0, 100),
                }
              : {}),
          }),
        },
      );
      setRecipient("");
      setItems((previous) => [
        result.conversation,
        ...previous.filter((item) => item.id !== result.conversation.id),
      ]);
      setSelected(result.conversation.id);
    } catch (reason) {
      setError(
        reason instanceof Error
          ? reason.message
          : "Could not start the conversation.",
      );
    } finally {
      setBusy(false);
    }
  }
  const active = items.find((item) => item.id === selected);
  const peer = active?.members.find((person) => person.id !== user.id);
  return (
    <>
      <PageHeading
        eyebrow="DIRECT MESSAGES"
        title="Conversations that last."
        subtitle="A feed for posts and conversation."
      />
      <div className="pl-messages">
        <section className="pl-conversation-list">
          <form className="pl-start-chat" onSubmit={start}>
            <label htmlFor="recipient">New conversation</label>
            <div>
              <input
                id="recipient"
                value={recipient}
                onChange={(event) => setRecipient(event.target.value)}
                placeholder="Username(s)"
              />
              <button
                className="pl-button outline"
                disabled={busy || !recipient.trim()}
              >
                Start
              </button>
            </div>
            <small className="pl-muted">
              For a group, separate usernames with commas.
            </small>
          </form>
          <h2>Your conversations</h2>
          {loading && <p className="pl-muted">Loading …</p>}
          {items.map((item) => {
            const other = item.members.find((person) => person.id !== user.id);
            return (
              <button
                key={item.id}
                className={`pl-conversation ${selected === item.id ? "active" : ""}`}
                onClick={() => {
                  setMessages([]);
                  setText("");
                  setSelected(item.id);
                }}
              >
                <Avatar user={other ?? item.members[0]} size="small" />
                <span>
                  <strong>
                    {item.name || other?.displayName || "Conversation"}
                  </strong>
                  <small>
                    {other && item.members.length === 2
                      ? `@${other.username}`
                      : `${item.members.length} Members`}
                  </small>
                </span>
              </button>
            );
          })}
          {!loading && items.length === 0 && (
            <p className="pl-muted">No conversations yet.</p>
          )}
        </section>
        <section className="pl-chat" aria-label="Messages">
          <header>
            {peer ? (
              <>
                <Avatar user={peer} size="small" />
                <div>
                  <strong>{active?.name || peer.displayName}</strong>
                  <small>
                    {active && active.members.length > 2
                      ? `${active.members.length} Members`
                      : `@${peer.username}`}
                  </small>
                </div>
                <button
                  className="pl-text-button pl-refresh"
                  onClick={() => void loadMessages()}
                >
                  Refresh
                </button>
              </>
            ) : (
              <p>Choose a conversation or start a new one.</p>
            )}
          </header>
          {error && <Notice>{error}</Notice>}
          <div className="pl-message-list">
            {nextMessageCursor && (
              <button
                className="pl-button quiet pl-older-messages"
                disabled={olderLoading}
                onClick={() => void loadOlderMessages()}
              >
                {olderLoading ? "Loading …" : "Load older messages"}
              </button>
            )}
            {messages.map((message) => (
              <article
                key={message.id}
                className={`pl-message ${message.sender.id === user.id ? "mine" : ""}`}
              >
                {active && active.members.length > 2 && (
                  <strong>
                    {message.sender.displayName || message.sender.username}
                  </strong>
                )}
                <p>{message.text}</p>
                <time>{when(message.createdAt)}</time>
              </article>
            ))}
            {active && messages.length === 0 && (
              <Empty
                title="A conversation begins here."
                body="Send the first message."
              />
            )}
          </div>
          <form className="pl-message-compose" onSubmit={send}>
            <label className="sr-only" htmlFor="message-text">
              Your message
            </label>
            <textarea
              id="message-text"
              value={text}
              onChange={(event) => setText(event.target.value)}
              maxLength={4000}
              placeholder="Write a message …"
              disabled={!active}
            />
            <button
              className="pl-button primary"
              disabled={!active || busy || !text.trim()}
            >
              {busy ? "Sending …" : "Send"}
            </button>
          </form>
        </section>
      </div>
    </>
  );
}
