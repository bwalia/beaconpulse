import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { brand } from "@/brand";

import { DocsShell } from "@/components/docs/shell";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("docs");
  return {
    title: {
      default: t("layoutTitleDefault", { brand: brand.name }),
      template: t("layoutTitleTemplate", { brand: brand.name }),
    },
    description: t("layoutDesc", { brand: brand.name }),
  };
}

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return <DocsShell>{children}</DocsShell>;
}
