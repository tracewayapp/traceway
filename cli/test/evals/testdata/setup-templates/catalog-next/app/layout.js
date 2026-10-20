export const metadata = {
  title: "Catalog",
  description: "Product catalog",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
