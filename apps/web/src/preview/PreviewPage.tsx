import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from "react";
import { demoPosts, type Evidence, type PreviewPost } from "./demoData.js";
import "./preview.css";

type Theme = "system" | "light" | "dark";
type IconName = "home" | "search" | "bookmark" | "settings" | "plus" | "arrow" | "spark" | "sun" | "moon" | "monitor" | "close" | "reply" | "chevron" | "menu" | "leaf" | "link";

function Icon({ name, size = 18 }: { name: IconName; size?: number }) {
  const paths: Record<IconName, ReactNode> = {
    home: <><path d="m3 10 9-7 9 7"/><path d="M5 9v11h14V9M9 20v-6h6v6"/></>, search: <><circle cx="10.8" cy="10.8" r="6.8"/><path d="m16 16 5 5"/></>, bookmark: <path d="M6 4h12v17l-6-4-6 4z"/>, settings: <><circle cx="12" cy="12" r="3"/><path d="m19.4 15 .1.1 1.2 1-1.2 2.1-1.5-.5a7.6 7.6 0 0 1-1.7 1l-.3 1.6h-2.4l-.3-1.6a7.6 7.6 0 0 1-1.7-1l-1.5.5-1.2-2.1 1.2-1a7 7 0 0 1 0-2l-1.2-1 1.2-2.1 1.5.5a7.6 7.6 0 0 1 1.7-1l.3-1.6h2.4l.3 1.6a7.6 7.6 0 0 1 1.7 1l1.5-.5 1.2 2.1-1.2 1a7 7 0 0 1-.1 1.9Z"/></>, plus: <><path d="M12 5v14M5 12h14"/></>, arrow: <><path d="M5 12h14M13 6l6 6-6 6"/></>, spark: <><path d="m12 3 1.7 5.3L19 10l-5.3 1.7L12 17l-1.7-5.3L5 10l5.3-1.7z"/><path d="m19 16 .8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z"/></>, sun: <><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></>, moon: <path d="M20.5 14A8 8 0 0 1 10 3.5 8.5 8.5 0 1 0 20.5 14Z"/>, monitor: <><rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/></>, close: <><path d="m6 6 12 12M18 6 6 18"/></>, reply: <><path d="M10 8 5 12l5 4"/><path d="M5 12h8a6 6 0 0 1 6 6"/></>, chevron: <path d="m9 18 6-6-6-6"/>, menu: <><path d="M4 6h16M4 12h16M4 18h16"/></>, leaf: <><path d="M20 4c-8 0-14 3-14 10a6 6 0 0 0 6 6c7 0 9-8 8-16Z"/><path d="M4 21c2-5 6-9 12-12"/></>, link: <><path d="M10 13a5 5 0 0 0 7 .1l2-2a5 5 0 0 0-7-7l-1.2 1.2"/><path d="M14 11a5 5 0 0 0-7-.1l-2 2a5 5 0 0 0 7 7l1.2-1.2"/></>,
  };
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}

function getSavedTheme(): Theme {
  try {
    const saved = localStorage.getItem("tassm-preview-theme");
    return saved === "light" || saved === "system" ? saved : "dark";
  } catch { return "dark"; }
}

function Avatar({ initials, tone, small = false }: { initials: string; tone: string; small?: boolean }) {
  return <span className={`pv-avatar pv-${tone}${small ? " pv-avatar-small" : ""}`} aria-hidden="true">{initials}</span>;
}

function PostCard({ post, selected, onSelect }: { post: PreviewPost; selected: boolean; onSelect: () => void }) {
  return <article className={`pv-post${selected ? " is-selected" : ""}`}>
    <button className="pv-post-main" onClick={onSelect} aria-label={`Open note by ${post.author}`} aria-pressed={selected}>
      <Avatar initials={post.initials} tone={post.tone} />
      <span className="pv-post-copy">
        <span className="pv-post-meta"><strong>{post.author}</strong><span>@{post.handle}</span><span className="pv-meta-dot">·</span><time>{post.time}</time></span>
        <span className="pv-topic">{post.topic}</span>
        <span className="pv-post-text">{post.text}</span>
        <span className="pv-post-foot"><span><Icon name="reply" size={16}/>{post.replies} replies</span><span className="pv-evidence-count"><Icon name="link" size={15}/>{post.evidence.length} {post.evidence.length === 1 ? "source" : "sources"}</span></span>
      </span>
      <span className="pv-post-chevron"><Icon name="chevron" size={17}/></span>
    </button>
  </article>;
}

function EvidenceCard({ item, selected, onClick }: { item: Evidence; selected: boolean; onClick: () => void }) {
  return <button className={`pv-evidence-card${selected ? " is-current" : ""}`} onClick={onClick} aria-pressed={selected}>
    <span className="pv-evidence-kind">{{ "Primary source": "Primary source", Context: "Context", Perspective: "Perspective" }[item.kind]}</span><strong>{item.title}</strong><span className="pv-evidence-source">{item.source}</span>
  </button>;
}

function EvidenceDetails({ item }: { item: Evidence }) {
  return <div className="pv-evidence-detail"><p className="pv-eyebrow">CONTEXT · {item.version}</p><dl><dt>Claim</dt><dd>{item.claim}</dd><dt>Finding</dt><dd className="pv-finding">{item.finding}</dd><dt>Source excerpt · {item.date}</dt><dd>{item.passage}</dd><dt>Counter-evidence</dt><dd>{item.contrary}</dd><dt>Limitations</dt><dd>{item.limitations}</dd></dl><span><Icon name="link" size={14}/>{item.source}</span></div>;
}

export default function PreviewPage() {
  const [theme, setTheme] = useState<Theme>(getSavedTheme);
  const [posts, setPosts] = useState(demoPosts);
  const [selectedId, setSelectedId] = useState(demoPosts[0].id);
  const [selectedEvidenceId, setSelectedEvidenceId] = useState(demoPosts[0].evidence[0].id);
  const [composeOpen, setComposeOpen] = useState(false);
  const [composerText, setComposerText] = useState("");
  const [mobileEvidenceOpen, setMobileEvidenceOpen] = useState(false);
  const [quietMode, setQuietMode] = useState(false);
  const composeRef = useRef<HTMLTextAreaElement>(null);
  const selectedPost = posts.find((post) => post.id === selectedId) ?? posts[0];
  const selectedEvidence = selectedPost?.evidence.find((item) => item.id === selectedEvidenceId) ?? selectedPost?.evidence[0];
  const visiblePosts = useMemo(() => quietMode ? [] : posts, [quietMode, posts]);

  useEffect(() => {
    try { localStorage.setItem("tassm-preview-theme", theme); } catch { /* Preview still works when storage is unavailable. */ }
  }, [theme]);

  useEffect(() => {
    if (!composeOpen && !mobileEvidenceOpen) return;
    const prior = document.activeElement as HTMLElement | null;
    const priorBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const focusTarget = composeOpen ? composeRef.current : document.querySelector<HTMLElement>(".pv-sheet-close");
    focusTarget?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setComposeOpen(false); setMobileEvidenceOpen(false); }
      if (event.key === "Tab" && focusTarget) {
        const dialog = focusTarget.closest("[role='dialog']");
        const focusables = dialog?.querySelectorAll<HTMLElement>("button:not([disabled]), textarea:not([disabled]), input:not([disabled]), [href]");
        if (!focusables?.length) return;
        const first = focusables[0]; const last = focusables[focusables.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => { document.removeEventListener("keydown", onKeyDown); document.body.style.overflow = priorBodyOverflow; prior?.focus(); };
  }, [composeOpen, mobileEvidenceOpen]);

  function selectPost(post: PreviewPost) {
    setSelectedId(post.id);
    setSelectedEvidenceId(post.evidence[0]?.id ?? "");
    setMobileEvidenceOpen(window.matchMedia("(max-width: 1050px)").matches);
  }

  function openSources() {
    if (window.matchMedia("(max-width: 1050px)").matches) setMobileEvidenceOpen(true);
    else document.getElementById("pv-context-title")?.focus();
  }

  function createPost(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const text = composerText.trim();
    if (!text) return;
    const post: PreviewPost = { id: `local-${Date.now()}`, author: "You", handle: "you", initials: "YO", tone: "blue", time: "just now", topic: "YOUR NOTE", text, replies: 0, evidence: [] };
    setPosts((previous) => [post, ...previous]); setSelectedId(post.id); setSelectedEvidenceId(""); setQuietMode(false); setComposerText(""); setComposeOpen(false);
  }

  return <div className="pv-app" data-theme={theme}>
    <div className="pv-frame">
      <aside className="pv-sidebar" aria-label="Main navigation">
        <a className="pv-brand" href="#feed" aria-label="TASSM home"><span className="pv-brand-mark"><Icon name="leaf" size={19}/></span><span>TASSM</span></a>
        <div className="pv-demo-mark"><span className="pv-live-dot"/> Preview <span>·</span> Demo</div>
        <nav className="pv-primary-nav">
          <a href="#feed" className="active"><Icon name="home"/> <span>Chronological feed</span></a>
          <button onClick={openSources}><Icon name="bookmark"/><span>View sources</span></button>
        </nav>
        <div className="pv-sidebar-rule"/><div className="pv-sidebar-note"><Icon name="spark" size={16}/><p>A small, private place for thoughtful conversation and context.</p></div>
        <div className="pv-sidebar-bottom"><div className="pv-profile-link"><span className="pv-user-avatar">DE</span><span><strong>Demo account</strong><small>Actually Social</small></span></div></div>
      </aside>

      <main id="feed" className="pv-main">
        <header className="pv-mobile-header"><a className="pv-brand" href="#feed"><span className="pv-brand-mark"><Icon name="leaf" size={18}/></span><span>TASSM</span></a><button className="pv-icon-button" aria-label="Open sources" onClick={() => setMobileEvidenceOpen(true)}><Icon name="bookmark"/></button></header>
        <div className="pv-feed-head"><div><p className="pv-eyebrow">TASSM PREVIEW <span>·</span> ACTUALLY SOCIAL</p><h1>The Reading Room</h1><p className="pv-subtitle">Ideas to think about. Sources to explore.</p></div><div className="pv-head-actions"><button className="pv-theme-button" aria-label="Change colour theme" title={`Colour theme: ${{ system: "System", light: "Light", dark: "Dark" }[theme]}`} onClick={() => setTheme(theme === "dark" ? "light" : theme === "light" ? "system" : "dark")}><Icon name={theme === "dark" ? "moon" : theme === "light" ? "sun" : "monitor"}/><span>{{ system: "System", light: "Light", dark: "Dark" }[theme]}</span></button><button className="pv-compose-button" onClick={() => setComposeOpen(true)}><Icon name="plus" size={17}/><span>Write a note</span></button></div></div>
        <div className="pv-feed-toolbar"><div className="pv-filter-tabs"><button className={!quietMode ? "active" : ""} onClick={() => setQuietMode(false)}>Demo posts <span>{String(posts.length).padStart(2, "0")}</span></button><button className={quietMode ? "active" : ""} onClick={() => setQuietMode(true)}>Empty state</button></div><span className="pv-sort-label">Chronological order</span></div>
        <section className="pv-feed" aria-label="Notes from your network">
          {visiblePosts.length ? visiblePosts.map((post) => <PostCard key={post.id} post={post} selected={post.id === selectedId} onSelect={() => selectPost(post)}/>) : <div className="pv-empty"><span className="pv-empty-icon"><Icon name="leaf" size={23}/></span><p className="pv-eyebrow">A MOMENT OF CALM</p><h2>It’s quiet here for now.</h2><p>A small network is not always busy. You can write the first note.</p><button className="pv-compose-button" onClick={() => setComposeOpen(true)}><Icon name="plus" size={16}/> Write the first note</button></div>}
          {visiblePosts.length > 0 && <div className="pv-feed-end"><span/><p>You’re all caught up</p><span/></div>}
        </section>
        <footer className="pv-mobile-nav" aria-label="Main navigation"><button className="active" aria-label="Chronological feed"><Icon name="home"/></button><button aria-label="View sources" onClick={openSources}><Icon name="bookmark"/></button><button className="pv-mobile-add" aria-label="Write a note" onClick={() => setComposeOpen(true)}><Icon name="plus"/></button><button aria-label={quietMode ? "Show demo posts" : "View empty state"} aria-pressed={quietMode} onClick={() => setQuietMode((current) => !current)}><Icon name="leaf"/></button><button aria-label="Change colour theme" onClick={() => setTheme(theme === "dark" ? "light" : theme === "light" ? "system" : "dark")}><Icon name={theme === "dark" ? "moon" : theme === "light" ? "sun" : "monitor"}/></button></footer>
      </main>

      <aside className="pv-context" aria-label="Note and sources">
        <div className="pv-context-top"><div><p className="pv-eyebrow">ABOUT THIS NOTE</p><h2 id="pv-context-title" tabIndex={-1}>{selectedPost ? "Context" : "Sources"}</h2></div><span className="pv-context-count">{selectedPost?.evidence.length ?? 0} sources</span></div>
        {selectedPost ? <>
          <div className="pv-selected-post"><div className="pv-selected-author"><Avatar initials={selectedPost.initials} tone={selectedPost.tone} small/><span><strong>{selectedPost.author}</strong><small>@{selectedPost.handle}</small></span></div><p>{selectedPost.text}</p></div>
          <div className="pv-context-divider"><span>ATTACHED SOURCES</span><span>IN THIS NOTE</span></div>
          {selectedPost.evidence.length ? <div className="pv-evidence-list">{selectedPost.evidence.map((item) => <EvidenceCard key={item.id} item={item} selected={item.id === (selectedEvidence?.id ?? "")} onClick={() => setSelectedEvidenceId(item.id)}/>)}</div> : <div className="pv-no-sources"><Icon name="bookmark" size={17}/><p>No sources yet</p><span>This demo note has no sources.</span></div>}
          {selectedEvidence && <EvidenceDetails item={selectedEvidence}/>}
        </> : <div className="pv-no-sources"><Icon name="bookmark" size={17}/><p>Select a note.</p><span>Sources and context appear here.</span></div>}
        <div className="pv-context-note"><Icon name="spark" size={15}/><span>Sources provide context. They are not a verdict.</span></div>
      </aside>
    </div>

    {composeOpen && <div className="pv-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setComposeOpen(false); }}><section className="pv-compose-dialog" role="dialog" aria-modal="true" aria-labelledby="pv-compose-title"><div className="pv-dialog-head"><div><p className="pv-eyebrow">A NOTE FOR THE FEED</p><h2 id="pv-compose-title">What would you like to share?</h2></div><button className="pv-icon-button" aria-label="Close editor" onClick={() => setComposeOpen(false)}><Icon name="close"/></button></div><form onSubmit={createPost}><label htmlFor="pv-compose-text">Your note</label><textarea ref={composeRef} id="pv-compose-text" value={composerText} onChange={(event) => setComposerText(event.target.value)} placeholder="A thought, an open question, or an interesting find …" maxLength={600} required/><div className="pv-compose-bottom"><span>{composerText.length} / 600 <span>·</span> Saved in this preview only</span><button className="pv-compose-button" type="submit" disabled={!composerText.trim()}><span>Share note</span><Icon name="arrow" size={16}/></button></div></form></section></div>}

    {mobileEvidenceOpen && <div className="pv-overlay pv-sheet-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setMobileEvidenceOpen(false); }}><section className="pv-evidence-sheet" role="dialog" aria-modal="true" aria-labelledby="pv-sheet-title"><div className="pv-sheet-grip"/><div className="pv-dialog-head"><div><p className="pv-eyebrow">ABOUT THIS NOTE</p><h2 id="pv-sheet-title">{selectedPost ? "Context" : "Sources"}</h2></div><button className="pv-icon-button pv-sheet-close" aria-label="Close sources" onClick={() => setMobileEvidenceOpen(false)}><Icon name="close"/></button></div>{selectedPost ? <><div className="pv-selected-post"><div className="pv-selected-author"><Avatar initials={selectedPost.initials} tone={selectedPost.tone} small/><span><strong>{selectedPost.author}</strong><small>@{selectedPost.handle}</small></span></div><p>{selectedPost.text}</p></div><div className="pv-context-divider"><span>ATTACHED SOURCES</span><span>{selectedPost.evidence.length} SOURCES</span></div>{selectedPost.evidence.length ? <>{selectedPost.evidence.map((item) => <EvidenceCard key={item.id} item={item} selected={item.id === (selectedEvidence?.id ?? "")} onClick={() => setSelectedEvidenceId(item.id)}/>)}{selectedEvidence && <EvidenceDetails item={selectedEvidence}/>}</> : <div className="pv-no-sources"><Icon name="bookmark" size={17}/><p>No sources yet</p><span>This demo note has no sources.</span></div>}</> : <p className="pv-no-sources">Select a note to see its sources.</p>}</section></div>}
  </div>;
}
