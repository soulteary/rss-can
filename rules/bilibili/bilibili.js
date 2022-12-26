const config = {
  URL: "https://www.bilibili.com/read/home",
  Mode: "mix",
  Title: ".article-title",
  Author: ".article-info-bar .up-content .nick-name",
  Category: ".article-info-bar .category",
  // DateTime: "(sub page)#app .title-container .article-read-info .publish-text",
  Description: ".article-desc",
  Link: ".article-title-holder",
  IdByProp: {
    object: "",
    prop: "data-id",
  },
};
potted.SetConfig(config).SetScope(".article-list .article-item");
