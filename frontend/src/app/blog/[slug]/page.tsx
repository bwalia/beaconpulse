import { PostRoute, postMetadata } from "@/components/cms/pages";

export const revalidate = 3600;

type Props = { params: Promise<{ slug: string }> };

// Posts render on first request and are then cached (no generateStaticParams): a new
// post is live as soon as it's published, without a rebuild.
export function generateMetadata(props: Props) {
  return postMetadata("blog", props);
}

export default function Page(props: Props) {
  return <PostRoute kind="blog" {...props} />;
}
