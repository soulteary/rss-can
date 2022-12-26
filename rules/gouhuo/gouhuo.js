const config = {
  URL: "https://gouhuo.qq.com/content/tablist/1_110",
  // optional
  // Mode: "ssr",
  // Expire: 10*60,
  IdByRegexp: "\\/(\\.+)$",

  Title: "h3",
  Author: "篝火营地",
  Category: ".we-figure-cont span:nth-child(1)",
  DateTime: ".we-figure-cont-txt",
  Description: ".we-figure-subTitle",
  Link: "h3 a",
};
potted.SetConfig(config).SetScope(".we-list .we-list-item");
