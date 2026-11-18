import Image from "next/image";

type Author = {
  name: string;
  url?: string;
  avatar: string;
  note?: string;
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
  paige: {
    name: "Paige Turner",
    avatar: "/images/paige-turner.jpg",
    note: "Paige is Traceway's AI writing agent. A person on the team checks every post before it is published.",
  },
};

const DEFAULT_AUTHOR_KEY = "dusan";

export function blogAuthor(author?: string): Author {
  return AUTHORS[author ?? DEFAULT_AUTHOR_KEY] ?? AUTHORS[DEFAULT_AUTHOR_KEY];
}

export function BlogByline({
  date,
  author,
  readingMinutes,
}: {
  date: string;
  author?: string;
  readingMinutes?: number;
}) {
  const author_record = blogAuthor(author);
  const className = author_record.note ? "blog-author blog-author-noted" : "blog-author";
  const inner = (
    <>
      <Image
        src={author_record.avatar}
        alt={author_record.name}
        width={32}
        height={32}
        className="blog-author-avatar"
      />
      <span className="blog-author-name">{author_record.name}</span>
      {author_record.note && (
        <span role="tooltip" id="blog-author-note" className="blog-author-note">
          {author_record.note}
        </span>
      )}
    </>
  );
  return (
    <div className="blog-byline">
      {author_record.url ? (
        <a
          href={author_record.url}
          target="_blank"
          rel="noopener noreferrer"
          className={className}
          aria-describedby={author_record.note ? "blog-author-note" : undefined}
        >
          {inner}
        </a>
      ) : (
        <span
          className={className}
          tabIndex={author_record.note ? 0 : undefined}
          aria-describedby={author_record.note ? "blog-author-note" : undefined}
        >
          {inner}
        </span>
      )}
      <span className="blog-byline-meta">
        {date && <time>{date}</time>}
        {date && readingMinutes && <span className="blog-byline-sep">·</span>}
        {readingMinutes && <span>{readingMinutes} min read</span>}
      </span>
    </div>
  );
}
