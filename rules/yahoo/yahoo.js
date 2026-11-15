const config = {
  Name: "Yahoo",
  // Use the US news feed: the portal redirects to a regional homepage.
  URL: "https://news.yahoo.com/rss/",
  IdByRegexp: "^(https?://[^#]+)",
  Title: "h2",
  Author: ".rss-author",
  DateTime: "time",
  Description: ".rss-description",
  Link: "a",
};
potted.SetConfig(config).SetScope("article");
