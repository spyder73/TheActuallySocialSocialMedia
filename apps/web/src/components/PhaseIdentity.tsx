import "./phase-identity.css";

/** Original Phase insignia: a shared circle around an open, faceted core. */
export function PhaseMark() {
  return (
    <svg className="phase-mark-art" viewBox="0 0 80 80" fill="none" aria-hidden="true">
      <path d="M35 4a36 36 0 0 0 0 72M45 4a36 36 0 0 1 0 72" stroke="currentColor" strokeWidth="2" />
      <path d="M22 58V22h27l11 11-11 12H33v13H22Zm11-24h12l3-3-3-3H33v6Z" fill="currentColor" />
      <path d="M27 65h26M31 70h18" stroke="currentColor" strokeWidth="2" />
    </svg>
  );
}

export function PhaseIdentity() {
  return (
    <div className="phase-identity">
      <div className="phase-identity-copy"><span className="phase-identity-label">INDEPENDENT VOICES / SHARED SPACE</span><strong>WE ARE<br/>PHASE.</strong></div>
      <div className="phase-identity-seal"><PhaseMark /></div>
    </div>
  );
}
