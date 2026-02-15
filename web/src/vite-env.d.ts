/**
 * Vite-specific module declarations
 *
 * These declarations allow TypeScript to understand Vite's special
 * import suffixes (e.g., ?inline for raw CSS text).
 */

declare module '*?inline' {
  const content: string;
  export default content;
}
