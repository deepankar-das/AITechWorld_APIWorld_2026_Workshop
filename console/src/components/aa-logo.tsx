/**
 * Author: Deepankar Das
 */

export function AALogo({ className = "h-8 w-8" }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" className={className}>
      <defs>
        <linearGradient id="logo-shield" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#22d3ee"/>
          <stop offset="100%" stopColor="#0891b2"/>
        </linearGradient>
      </defs>
      {/* Shield outer */}
      <path d="M16 2L4 7v8c0 7.7 5.1 14.2 12 16 6.9-1.8 12-8.3 12-16V7L16 2z" fill="url(#logo-shield)" opacity="0.9"/>
      {/* Shield inner */}
      <path d="M16 4.5L6.5 8.5v6.5c0 6.3 4.1 11.7 9.5 13.2 5.4-1.5 9.5-6.9 9.5-13.2V8.5L16 4.5z" fill="#0f172a"/>

      {/* Main large center tongue */}
      <path d="M16 6.5 C15 8.5 14 10 13.5 12 C13 14 13.2 15.5 14 17 C14.5 18 15.2 18.5 16 18.5 C16.8 18.5 17.5 18 18 17 C18.8 15.5 19 14 18.5 12 C18 10 17 8.5 16 6.5Z" fill="#f97316"/>
      <path d="M16 6.5 C15 8.5 14 10 13.5 12 C13 14 13.2 15.5 14 17 C14.5 18 15.2 18.5 16 18.5 C16.8 18.5 17.5 18 18 17 C18.8 15.5 19 14 18.5 12 C18 10 17 8.5 16 6.5Z" fill="none" stroke="#fbbf24" strokeWidth="0.4"/>

      {/* Left medium tongue */}
      <path d="M14 11 C13 12.5 11.5 14 11.2 15.5 C11 16.5 11.5 17.2 12.2 17.5 C12.8 17.7 13.3 17.3 13.5 16.5 C13.8 15.2 14 13.5 14 11Z" fill="#f97316" opacity="0.85"/>
      <path d="M14 11 C13 12.5 11.5 14 11.2 15.5 C11 16.5 11.5 17.2 12.2 17.5 C12.8 17.7 13.3 17.3 13.5 16.5 C13.8 15.2 14 13.5 14 11Z" fill="none" stroke="#eab308" strokeWidth="0.4"/>

      {/* Right medium tongue */}
      <path d="M18 11 C19 12.5 20.5 14 20.8 15.5 C21 16.5 20.5 17.2 19.8 17.5 C19.2 17.7 18.7 17.3 18.5 16.5 C18.2 15.2 18 13.5 18 11Z" fill="#f97316" opacity="0.85"/>
      <path d="M18 11 C19 12.5 20.5 14 20.8 15.5 C21 16.5 20.5 17.2 19.8 17.5 C19.2 17.7 18.7 17.3 18.5 16.5 C18.2 15.2 18 13.5 18 11Z" fill="none" stroke="#eab308" strokeWidth="0.4"/>

      {/* Left small tongue */}
      <path d="M12.5 13.5 C11.8 14.5 10.8 15.5 10.8 16.3 C10.8 16.8 11.2 17.1 11.6 17 C12 16.9 12.3 16.3 12.5 15.5 C12.6 14.8 12.6 14 12.5 13.5Z" fill="#eab308" opacity="0.7"/>
      <path d="M12.5 13.5 C11.8 14.5 10.8 15.5 10.8 16.3 C10.8 16.8 11.2 17.1 11.6 17 C12 16.9 12.3 16.3 12.5 15.5 C12.6 14.8 12.6 14 12.5 13.5Z" fill="none" stroke="#fbbf24" strokeWidth="0.3"/>

      {/* Right small tongue */}
      <path d="M19.5 13.5 C20.2 14.5 21.2 15.5 21.2 16.3 C21.2 16.8 20.8 17.1 20.4 17 C20 16.9 19.7 16.3 19.5 15.5 C19.4 14.8 19.4 14 19.5 13.5Z" fill="#eab308" opacity="0.7"/>
      <path d="M19.5 13.5 C20.2 14.5 21.2 15.5 21.2 16.3 C21.2 16.8 20.8 17.1 20.4 17 C20 16.9 19.7 16.3 19.5 15.5 C19.4 14.8 19.4 14 19.5 13.5Z" fill="none" stroke="#fbbf24" strokeWidth="0.3"/>

      {/* Inner bright core */}
      <path d="M16 10 C15.3 11.5 14.8 13 14.8 14.2 C14.8 15 15.2 15.8 15.7 16 C15.9 16.1 16.1 16.1 16.3 16 C16.8 15.8 17.2 15 17.2 14.2 C17.2 13 16.7 11.5 16 10Z" fill="#fbbf24"/>

      {/* Hottest white tip */}
      <path d="M16 12.5 C15.6 13.3 15.3 14 15.3 14.5 C15.3 14.9 15.6 15.2 16 15.2 C16.4 15.2 16.7 14.9 16.7 14.5 C16.7 14 16.4 13.3 16 12.5Z" fill="#fef9c3"/>

      {/* AA text */}
      <text x="16" y="24.5" textAnchor="middle" fontFamily="Arial,sans-serif" fontWeight="bold" fontSize="5" fill="#22d3ee" letterSpacing="0.5">AA</text>
    </svg>
  );
}