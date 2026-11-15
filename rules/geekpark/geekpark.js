const config = {
  Name: "极客公园",
  // The homepage blocks automated requests; the official RSS remains public.
  URL: "https://www.geekpark.net/rss",
  IdByRegexp: "/news/([0-9]+)",
  Title: "h2",
  Author: ".rss-author",
  DateTime: "time",
  Description: ".rss-description",
  Link: "a",
};
potted.SetConfig(config).SetScope("article");
