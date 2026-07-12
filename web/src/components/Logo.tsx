import { useId } from 'react';

// Mochi brand mark: a five-petal sakura on a soft pink rounded square.
export function Logo({ size = 40, className = '' }: { size?: number; className?: string }) {
  // SVG paint-server IDs are document-global. A unique ID prevents the
  // hidden desktop logo from stealing the mobile logo's gradient in WebViews.
  const gradientId = `mochi-bg-${useId().replace(/:/g, '')}`;

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      className={className}
      role="img"
      aria-label="Mochi"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ff8fb3" />
          <stop offset="100%" stopColor="#f2528b" />
        </linearGradient>
      </defs>
      <rect x="2" y="2" width="60" height="60" rx="19" fill={`url(#${gradientId})`} />
      <g transform="translate(32 33)">
        {[0, 72, 144, 216, 288].map((deg) => (
          <path
            key={deg}
            d="M0 -17 C 6 -12.5, 6 -5, 0 -1.5 C -6 -5, -6 -12.5, 0 -17"
            fill="#ffffff"
            fillOpacity="0.94"
            transform={`rotate(${deg})`}
          />
        ))}
        <circle r="4.2" fill="#ffdf8a" />
      </g>
    </svg>
  );
}
