const config = {
  Name: "36氪",
  // The homepage now returns a WAF challenge to non-browser clients.
  // www.36kr.com/feed is the public article feed.
  URL: "https://www.36kr.com/feed",
  IdByRegexp: "/(?:p|newsflashes)/([0-9]+)",

  Title: "h2",
  Author: ".rss-author",
  DateTime: "time",
  Description: ".rss-description",
  Link: "a",
};
potted.SetConfig(config).SetScope("article");
