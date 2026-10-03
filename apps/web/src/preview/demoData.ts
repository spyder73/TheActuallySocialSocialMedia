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
    time: "vor 12 Min.",
    topic: "LEKTÜRENOTIZ",
    text: "Eine kleine Erinnerung aus der heutigen Lektüre: Eine aufgeräumte Grafik kann eine offene Frage geklärt wirken lassen. Zwischen einer Zahl und ihrer Bedeutung möchte ich etwas mehr Platz lassen.",
    replies: 0,
    evidence: [
      { id: "e1", source: "TASSM-Demoarchiv · Beleg A", title: "Zur Gestaltung von Diagrammen", detail: "Kontext zum markierten Satz in diesem fiktiven Beispiel.", kind: "Context", claim: "„Das Beispieldiagramm verwendet eine verkürzte vertikale Achse.“", finding: "FIKTIVES DEMO · Die angeführte Beispieldarstellung verwendet tatsächlich eine verkürzte Achse. Dies ist ein erfundenes Lehrbeispiel, keine Prüfung eines realen Sachverhalts.", passage: "„Für bessere Lesbarkeit beginnt die dargestellte Achse bei 40 Einheiten.“ — TASSM-Demoarchiv, Beleg A, Absatz 2", date: "14. Sep. 2026", contrary: "Das Beispiel enthält außerdem einen Hinweis zur Achsenskalierung.", limitations: "Archiv, Grafik und Passage sind erfunden. Es wurden keine externen Quellen geprüft.", version: "Demo-Fixture · v1" },
      { id: "e2", source: "Mara’s Notizbuch · Beispiel", title: "Fragen vor Schlussfolgerungen", detail: "Persönliche Notizen dazu, was gemessen wurde, wer einbezogen war und was offen bleibt.", kind: "Perspective", claim: "„Eine aufgeräumte Grafik kann eine offene Frage geklärt wirken lassen.“", finding: "FIKTIVES DEMO · Das ist eine persönliche Einschätzung und keine überprüfbare Tatsachenbehauptung.", passage: "„Zuerst fragen: Was wurde gemessen, wer war dabei, was ist noch offen?“ — Beispielnotiz", date: "03. Okt. 2026", contrary: "Zu einer Meinung gibt es hier keine Gegenbelege.", limitations: "Eintrag und Autorin sind illustrative Demo-Inhalte.", version: "Demo-Fixture · v1" },
    ],
  },
  {
    id: "slow-news",
    author: "Jonah Park",
    handle: "jonahpark",
    initials: "JP",
    tone: "clay",
    time: "vor 38 Min.",
    topic: "OFFENE FRAGE",
    text: "Wenn sich eine Geschichte noch entwickelt: Wie bleibst du informiert, ohne jedes Update gleich als neue Schlussfolgerung zu lesen? Ich notiere, was sich geändert hat und was weiterhin offen ist.",
    replies: 0,
    evidence: [
      { id: "e3", source: "TASSM-Demoarchiv · Leitfaden B", title: "Eine laufende Geschichte verfolgen", detail: "Eine Musterquelle zur fiktiven Aussage in diesem Demo.", kind: "Primary source", claim: "„Der Leitfaden empfiehlt, Updates getrennt von offenen Fragen zu notieren.“", finding: "FIKTIVES DEMO · Die Beispielpassage stützt die Formulierung, die diesem erfundenen Leitfaden zugeschrieben wird.", passage: "„Führe eine datierte Liste neuer Informationen und getrennt davon eine Liste offener Fragen.“ — TASSM-Demoarchiv, Leitfaden B, Abschnitt 1", date: "22. Aug. 2026", contrary: "Dieses Fixture enthält keine Gegenpassage.", limitations: "Der Leitfaden ist erfunden. Das Beispiel bewertet keine reale aktuelle Geschichte.", version: "Demo-Fixture · v1" },
      { id: "e4", source: "Notizen am Tisch · Beispiel", title: "Was sich änderte und was offen blieb", detail: "Eine fiktive gemeinsame Notiz mit Zeitlinie und offenen Fragen.", kind: "Context", claim: "„Die Geschichte entwickelt sich noch.“", finding: "FIKTIVES DEMO · Die Informationen in diesem Beispiel reichen nicht aus, um die Aussage zu beurteilen.", passage: "„Update 2: Ein Detail hat sich geändert; zwei Fragen bleiben offen.“ — Beispielnotiz", date: "02. Okt. 2026", contrary: "Im Beispiel steht außerdem, dass ein früheres Detail korrigiert wurde.", limitations: "Diese Notiz hat keine reale Quellenkette und ist keine aktuelle Information.", version: "Demo-Fixture · v1" },
    ],
  },
  {
    id: "shared-table",
    author: "Ari Okafor",
    handle: "ari.o",
    initials: "AO",
    tone: "blue",
    time: "vor 1 Std.",
    topic: "ZUSAMMENLESEN",
    text: "Fürs lange Wochenende sammeln wir eine kleine Leseliste. Am hilfreichsten sind bisher die Beiträge, die erklären, woher sie etwas wissen – nicht nur, was sie denken.",
    replies: 0,
    evidence: [
      { id: "e5", source: "TASSM-Demoarchiv · Leitfaden C", title: "Eine Quelle lesen", detail: "Ein fiktiver Leitfaden zu Autorenschaft, Kontext und Quellenketten.", kind: "Context", claim: "„Der Leitfaden empfiehlt, Quellenverweisen zu folgen.“", finding: "FIKTIVES DEMO · Die Beispielpassage entspricht der Aussage über diesen erfundenen Leitfaden.", passage: "„Öffne das zitierte Werk und verfolge seine Verweise bis zum Ausgangsmaterial.“ — TASSM-Demoarchiv, Leitfaden C, Absatz 4", date: "18. Jul. 2026", contrary: "Dieses Fixture enthält keine Gegenpassage.", limitations: "Leitfaden und Zitat wurden für diese Vorschau erfunden.", version: "Demo-Fixture · v1" },
    ],
  },
];
