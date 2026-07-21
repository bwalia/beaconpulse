/**
 * The product name, in one place.
 *
 * Translated strings deliberately do NOT bake the name in — they carry a `{brand}`
 * placeholder (the same convention the auth screens already use) so the prose reads
 * naturally in every language while the name stays a single constant. When the
 * white-label branding work lands, this becomes `brand.name` and nothing else has to
 * change: every call site already reads it from here.
 */
export const BRAND_NAME = "Beacon Pulse";
