import { ListingRoute, listingMetadata, pageNumber } from "@/components/cms/pages";

export const revalidate = 3600;

type Props = { params: Promise<{ page: string }> };

export async function generateMetadata(props: Props) {
  return listingMetadata("articles", await pageNumber("articles", props));
}

export default async function Page(props: Props) {
  return <ListingRoute kind="articles" page={await pageNumber("articles", props)} />;
}
