import Image from "next/image";

type Author = {
  name: string;
  url: string;
  avatar: string;
};

const AUTHORS: Record<string, Author> = {
  dusan: {
    name: "Dusan Stanojevic",
    url: "https://www.linkedin.com/in/dusanstanojeviccs",
    avatar: "/images/dusan-stanojevic.jpg",
  },
  jovan: {
    name: "Jovan Stojiljkovic",
    url: "https://github.com/jstojiljkovic",
    avatar: "/images/jovan-stojiljkovic.jpg",
  },
};

const DEFAULT_AUTHOR_KEY = "dusan";

export function BlogByline({ date, author }: { date: string; author?: string }) {
  const author_record = AUTHORS[author ?? DEFAULT_AUTHOR_KEY] ?? AUTHORS[DEFAULT_AUTHOR_KEY];
  return (
    <div className="blog-byline">
      <a
        href={author_record.url}
        target="_blank"
        rel="noopener noreferrer"
        className="blog-author"
      >
        <Image
          src={author_record.avatar}
          alt={author_record.name}
          width={32}
          height={32}
          className="blog-author-avatar"
        />
        {author_record.name}
      </a>
      {date && (
        <>
          <span className="blog-byline-sep">·</span>
          <span>{date}</span>
        </>
      )}
    </div>
  );
}
