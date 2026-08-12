import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: ["/", "/login", "/signup"],
      // The dashboard itself is per-merchant private data behind auth —
      // nothing in there is meant to be indexed even if a crawler
      // somehow got a session.
      disallow: "/dashboard",
    },
    sitemap: "https://conduit-dashboard.onrender.com/sitemap.xml",
  };
}
