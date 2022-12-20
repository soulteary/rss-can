const config = {
  URL: "https://www.bilibili.com/read/home",
  Mode: "csr",
  Title: ".article-title",
  Author: ".article-info-bar .up-content .nick-name",
  Category: ".article-info-bar .category",
  // DateTime: "(sub page)#app .title-container .article-read-info .publish-text",
  Description: ".article-desc",
  Link: ".article-title-holder",
};
potted.SetConfig(config).SetScope(".article-list .article-item");
