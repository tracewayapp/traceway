import { useTheme } from "nextra-theme-docs";

export default {
  logo: function Logo() {
    const { resolvedTheme } = useTheme();
    return (
      <img
        src={
          resolvedTheme === "dark"
            ? "/traceway-logo-white.png"
            : "/traceway-logo.png"
        }
        alt="Traceway"
        style={{ height: "32px" }}
      />
    );
  },
  project: {
    link: "https://github.com/tracewayapp/traceway",
  },
  docsRepositoryBase: "https://github.com/tracewayapp/traceway/tree/main/docs",
  footer: {
    text: `${new Date().getFullYear()} Traceway. All rights reserved.`,
  },
  useNextSeoProps() {
    return {
      titleTemplate: "%s - Traceway Docs",
    };
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta
        name="description"
        content="Traceway - Error tracking and monitoring platform"
      />
    </>
  ),
  primaryHue: 205,
  darkMode: true,
  nextThemes: {
    defaultTheme: "light",
  },
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
  },
  toc: {
    backToTop: true,
  },
  editLink: {
    text: "Edit this page on GitHub",
  },
  feedback: {
    content: null,
  },
};
