import { ListingRoute, listingMetadata, pageNumber } from "@/components/cms/pages";

export const revalidate = 3600;

type Props = { params: Promise<{ page: string }> };

export async function generateMetadata(props: Props) {
  return listingMetadata("blog", await pageNumber("blog", props));
}

export default async function Page(props: Props) {
  return <ListingRoute kind="blog" page={await pageNumber("blog", props)} />;
}
