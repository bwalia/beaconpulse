import { ListingRoute, listingMetadata } from "@/components/cms/pages";

// Content comes from the OpsAPI CMS through the tagged, time-revalidated data cache in
// lib/cms.ts; the OpsAPI webhook (/cms/revalidate) busts it on publish.
export const revalidate = 3600;

export function generateMetadata() {
  return listingMetadata("articles");
}

export default function Page() {
  return <ListingRoute kind="articles" />;
}
