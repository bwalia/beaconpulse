# Translations

One JSON file per language, keyed identically to `en.json` (the source). The app reads
the active locale from a cookie the language switcher sets; a missing key falls back to
English, so a partially-translated language shows English for the gaps rather than
breaking.

## What is translated

The **UI chrome** — navigation, auth, common buttons, theme and language labels. These
are short, standard software terms that translate cleanly.

Long-form prose (the docs, detailed dashboard help text) is intentionally **not** here.
Machine-translating technical prose into 16 languages produces confident nonsense that
is worse than English; those strings stay in English until a human translates them.
When they do, add the namespace here and wire it up — the infrastructure already
carries it.

## Add a language

1. Copy `en.json` to `<code>.json` and translate the values (keep the keys and the
   `{brand}` placeholders).
2. Add `<code>` to `LOCALES` in `src/i18n/config.ts`.
3. If it is right-to-left, add it to the `RTL` set in the same file.

That is all — the switcher, `<html lang/dir>`, and message loading all read from that
list.

## Add a translatable string

1. Add the key to `en.json` (and ideally the others; untranslated keys fall back to
   English).
2. In a component: `const t = useTranslations("namespace"); … {t("key")}`.
