const config = {
  Name: "智源社区",
  URL: "https://hub.baai.ac.cn/",
  // optional
  // Mode: "ssr",
  // Expire: 10*60,
  IdByRegexp: "\\/(\\d+)$",

  Title: ".story-item-title",
  Author: ".story-item-author-name",
  Category: "最热",
  DateTime: ".story-item-time",
  Description: ".story-item-summary",
  Link: ".story-item-main > a",
};
potted.SetConfig(config).SetScope("main .story-list .story-list-container");
