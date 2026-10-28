// A full navigation keeps credentials and OIDC flow on their own origins.
export const loginNavigation = { open: (url: string) => window.location.replace(url) }
