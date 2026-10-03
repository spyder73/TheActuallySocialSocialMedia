export type Evidence = {
  id: string;
  source: string;
  title: string;
  detail: string;
  kind: "Primary source" | "Context" | "Perspective";
  claim: string;
  finding: string;
  passage: string;
  date: string;
  contrary: string;
  limitations: string;
  version: string;
};

export type PreviewPost = {
  id: string;
  author: string;
  handle: string;
  initials: string;
  tone: string;
  time: string;
  topic: string;
  text: string;
  replies: number;
  evidence: Evidence[];
};

// Every entry is fictional interface copy for preview purposes, not a factual claim.
export const demoPosts: PreviewPost[] = [
  {
    id: "field-notes",
    author: "Mara Ellison",
    handle: "mara.e",
    initials: "ME",
    tone: "sage",
    time: "12 min ago",
    topic: "READING NOTE",
    text: "A small reminder from today’s reading: a tidy chart can make an open question look settled. I want to leave more space between a number and what it means.",
    replies: 0,
    evidence: [
      { id: "e1", source: "TASSM Demo Archive · Exhibit A", title: "On chart design", detail: "Context for the highlighted sentence in this fictional example.", kind: "Context", claim: "“The sample chart uses a truncated vertical axis.”", finding: "FICTIONAL DEMO · The cited sample chart does use a truncated axis. This is an invented teaching example, not an assessment of a real-world claim.", passage: "“For readability, the displayed axis starts at 40 units.” — TASSM Demo Archive, Exhibit A, paragraph 2", date: "14 Sep 2026", contrary: "The example also notes the axis scale.", limitations: "The archive, chart, and passage are fictional. No external sources were checked.", version: "Demo fixture · v1" },
      { id: "e2", source: "Mara’s Notebook · Example", title: "Questions before conclusions", detail: "Personal notes on what was measured, who was included, and what remains unknown.", kind: "Perspective", claim: "“A tidy chart can make an open question look settled.”", finding: "FICTIONAL DEMO · This is a personal opinion, not a verifiable factual claim.", passage: "“First ask: What was measured, who took part, and what remains unknown?” — Sample note", date: "3 Oct 2026", contrary: "There is no counter-evidence to an opinion here.", limitations: "The entry and author are illustrative demo content.", version: "Demo fixture · v1" },
    ],
  },
  {
    id: "slow-news",
    author: "Jonah Park",
    handle: "jonahpark",
    initials: "JP",
    tone: "clay",
    time: "38 min ago",
    topic: "OPEN QUESTION",
    text: "When a story is still unfolding, how do you stay informed without treating every update as a new conclusion? I note what has changed and what remains open.",
    replies: 0,
    evidence: [
      { id: "e3", source: "TASSM Demo Archive · Guide B", title: "Following an unfolding story", detail: "A sample source for the fictional claim in this demo.", kind: "Primary source", claim: "“The guide recommends recording updates separately from open questions.”", finding: "FICTIONAL DEMO · The sample passage supports the wording attributed to this fictional guide.", passage: "“Keep a dated list of new information and a separate list of open questions.” — TASSM Demo Archive, Guide B, section 1", date: "22 Aug 2026", contrary: "This fixture contains no counter-passage.", limitations: "The guide is fictional. This example does not assess a real current event.", version: "Demo fixture · v1" },
      { id: "e4", source: "Notes at the Table · Example", title: "What changed and what remains open", detail: "A fictional shared note with a timeline and open questions.", kind: "Context", claim: "“The story is still unfolding.”", finding: "FICTIONAL DEMO · There is not enough information in this example to assess the claim.", passage: "“Update 2: One detail changed; two questions remain open.” — Sample note", date: "2 Oct 2026", contrary: "The example also says an earlier detail was corrected.", limitations: "This note has no real source trail and is not current information.", version: "Demo fixture · v1" },
    ],
  },
  {
    id: "shared-table",
    author: "Ari Okafor",
    handle: "ari.o",
    initials: "AO",
    tone: "blue",
    time: "1 hour ago",
    topic: "READING TOGETHER",
    text: "We’re putting together a short reading list for the long weekend. The most helpful posts so far explain how their authors know something, not just what they think.",
    replies: 0,
    evidence: [
      { id: "e5", source: "TASSM Demo Archive · Guide C", title: "Reading a source", detail: "A fictional guide to authorship, context, and source trails.", kind: "Context", claim: "“The guide recommends following source references.”", finding: "FICTIONAL DEMO · The sample passage matches the claim about this fictional guide.", passage: "“Open the cited work and follow its references back to the original material.” — TASSM Demo Archive, Guide C, paragraph 4", date: "18 Jul 2026", contrary: "This fixture contains no counter-passage.", limitations: "The guide and quotation were invented for this preview.", version: "Demo fixture · v1" },
    ],
  },
];
