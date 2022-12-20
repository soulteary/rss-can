const config = {
  URL: "https://36kr.com/",
  // optional
  // Mode: "ssr",
  Expire: 10*60,
  Title: ".article-item-title",
  Author: ".kr-flow-bar-author",
  Category: ".kr-flow-bar-motif a",
  DateTime: ".kr-flow-bar-time",
  Description: ".article-item-description",
  Link: ".article-item-title",
};
potted.SetConfig(config).SetScope("#app .main-right .kr-home-main .kr-home-flow .kr-home-flow-list .kr-flow-article-item");
