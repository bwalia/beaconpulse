// The languages the product ships in.
//
// Adding one is: translate src/messages/en.json into src/messages/<code>.json, then add
// the code here. Nothing else — the switcher, the request config and the <html lang>
// all read this list.
//
// Order is display order in the switcher. English first (the source), then by reach.
export const LOCALES = [
  { code: "en", name: "English", native: "English" },
  { code: "es", name: "Spanish", native: "Español" },
  { code: "fr", name: "French", native: "Français" },
  { code: "de", name: "German", native: "Deutsch" },
  { code: "pt", name: "Portuguese", native: "Português" },
  { code: "it", name: "Italian", native: "Italiano" },
  { code: "nl", name: "Dutch", native: "Nederlands" },
  { code: "pl", name: "Polish", native: "Polski" },
  { code: "tr", name: "Turkish", native: "Türkçe" },
  { code: "ru", name: "Russian", native: "Русский" },
  { code: "uk", name: "Ukrainian", native: "Українська" },
  { code: "ja", name: "Japanese", native: "日本語" },
  { code: "ko", name: "Korean", native: "한국어" },
  { code: "zh", name: "Chinese (Simplified)", native: "简体中文" },
  { code: "hi", name: "Hindi", native: "हिन्दी" },
  { code: "ar", name: "Arabic", native: "العربية" },
] as const;

export type LocaleCode = (typeof LOCALES)[number]["code"];

export const DEFAULT_LOCALE: LocaleCode = "en";

/** The cookie the chosen locale is stored in. Read server-side to pick messages. */
export const LOCALE_COOKIE = "BEACON_LOCALE";

const CODES = new Set(LOCALES.map((l) => l.code));

/** Narrows an arbitrary string to a supported locale, falling back to the default. */
export function resolveLocale(value: string | undefined | null): LocaleCode {
  return value && CODES.has(value as LocaleCode) ? (value as LocaleCode) : DEFAULT_LOCALE;
}

// Right-to-left languages. Setting dir on <html> gives correct text flow for these; the
// browser mirrors inline direction natively. Arabic is the one shipped RTL language.
const RTL = new Set<LocaleCode>(["ar"]);

/** "rtl" for right-to-left languages, else "ltr". Drives the <html dir> attribute. */
export function localeDir(code: LocaleCode): "rtl" | "ltr" {
  return RTL.has(code) ? "rtl" : "ltr";
}
