import "./phase-diagram.css";

export function PhaseMark() {
  return (
    <svg className="phase-mark-art" viewBox="0 0 40 40" fill="none" aria-hidden="true">
      <path className="phase-curve" d="M7 30C12 27 12 13 20 12s9 12 13 7" />
      <path className="phase-boundary" d="M8 33C16 28 15 17 22 16s8 8 12 5" />
      <circle className="phase-point" cx="10" cy="27" r="1.7" />
      <circle className="phase-point" cx="15" cy="20" r="1.4" />
      <circle className="phase-point" cx="18" cy="13" r="1.6" />
      <circle className="phase-point" cx="24" cy="18" r="1.4" />
      <circle className="phase-point" cx="29" cy="22" r="1.7" />
    </svg>
  );
}

export function PhaseBanner() {
  return (
    <div className="phase-banner" aria-label="Individual voices. Collective states.">
      <svg className="phase-diagram-art" viewBox="0 0 184 76" fill="none" aria-hidden="true">
        <path className="phase-grid" d="M8 12H176M8 31H176M8 50H176M8 69H176M20 7V70M60 7V70M100 7V70M140 7V70" />
        <path className="phase-boundary" d="M8 61C37 59 43 44 62 42S90 48 105 36s24-23 42-23h29" />
        <path className="phase-curve" d="M8 48c22-1 25-14 45-14s26 13 43 8 23-20 42-20 25 8 38 5" />
        <g className="phase-cluster"><circle cx="40" cy="48" r="2"/><circle cx="47" cy="44" r="1.7"/><circle cx="50" cy="51" r="1.5"/><circle cx="56" cy="46" r="1.8"/><circle cx="61" cy="40" r="1.4"/></g>
        <g className="phase-cluster"><circle cx="116" cy="31" r="1.8"/><circle cx="122" cy="26" r="1.5"/><circle cx="128" cy="33" r="1.8"/><circle cx="133" cy="27" r="1.3"/><circle cx="139" cy="31" r="1.6"/></g>
      </svg>
      <span>Individual voices. Collective states.</span>
    </div>
  );
}
