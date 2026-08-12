import type { MetadataRoute } from "next";

const SITE_URL = "https://conduit-dashboard.onrender.com";

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: SITE_URL, changeFrequency: "monthly", priority: 1 },
    { url: `${SITE_URL}/login`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${SITE_URL}/signup`, changeFrequency: "yearly", priority: 0.5 },
  ];
}
